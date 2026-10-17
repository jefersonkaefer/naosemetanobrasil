package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pquerna/otp/totp"
	"golang.org/x/term"
	"naosemeta.com/backend/internal/adapters/postgres"
	"naosemeta.com/backend/internal/adapters/security"
	"naosemeta.com/backend/internal/config"
	"naosemeta.com/backend/internal/domain"
)

func main() {
	if err := run(); err != nil {
		log.Print("Operação administrativa não concluída. Nenhuma alteração parcial foi aplicada.")
		os.Exit(1)
	}
}
func run() error {
	name := flag.String("username", "", "new administrator username")
	promote := flag.Bool("promote", false, "promote an existing member after password and authenticator verification")
	flag.Parse()
	username, err := domain.NormalizeUsername(*name)
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	restore, err := privateTerminal()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Execute o comando em um terminal privado interativo; não informe senhas pelo chat.")
		return err
	}
	defer restore()
	if *promote {
		return promoteExisting(cfg, username)
	}
	password, err := readSecret("Senha (mínimo de 12 caracteres): ")
	if err != nil {
		return err
	}
	if err = domain.ValidatePassword(password); err != nil {
		return err
	}
	confirmation, err := readSecret("Repita a senha: ")
	if err != nil || password != confirmation {
		return domain.ErrInvalid
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "Não se Meta no Brasil", AccountName: username})
	if err != nil {
		return err
	}
	fmt.Println("Adicione este endereço ao seu autenticador. Não compartilhe:")
	fmt.Println(key.URL())
	code, err := readSecret("Código atual do autenticador: ")
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if !totp.Validate(strings.TrimSpace(code), key.Secret()) {
		return domain.ErrUnauthenticated
	}
	second, err := security.NewSecondFactor(cfg.TOTPKey)
	if err != nil {
		return err
	}
	passwords, err := security.NewPasswords()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	hash, err := passwords.Hash(ctx, password)
	if err != nil {
		return err
	}
	secrets := security.NewSecrets(cfg.Pepper)
	codes, hashes := make([]string, 10), make([]string, 10)
	for i := range codes {
		codes[i] = "NSR-" + secrets.Token(16)
		hashes[i] = secrets.Digest("recovery:" + codes[i])
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	user := domain.User{ID: secrets.NewID(), Username: username, Role: "admin", CreatedAt: now}
	if err = postgres.New(pool).CreateAccount(ctx, domain.Credential{User: user, PasswordHash: hash, TOTPEncrypted: second.Seal(key.Secret())}, hashes); err != nil {
		return err
	}
	fmt.Println("Administrador criado. Guarde estes códigos de recuperação em local privado:")
	for _, value := range codes {
		fmt.Println(value)
	}
	return nil
}
func readSecret(prompt string) (string, error) {
	fmt.Print(prompt)
	value, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	return string(value), err
}
