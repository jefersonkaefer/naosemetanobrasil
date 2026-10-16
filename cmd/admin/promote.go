package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pquerna/otp/totp"
	"naosemeta.com/backend/internal/adapters/postgres"
	"naosemeta.com/backend/internal/adapters/security"
	"naosemeta.com/backend/internal/application"
	"naosemeta.com/backend/internal/config"
)

func promoteExisting(cfg config.Config, username string) error {
	fmt.Printf("Ativar acesso administrativo para %s neste ambiente.\n", username)
	password, err := readSecret("Senha atual da sua conta (não será alterada): ")
	if err != nil {
		return err
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "Não se Meta no Brasil", AccountName: username})
	if err != nil {
		return err
	}
	fmt.Println("Adicione esta chave ao seu autenticador; não compartilhe nem envie no chat:")
	fmt.Println(key.Secret())
	fmt.Println("Tipo: TOTP, 6 dígitos, período de 30 segundos.")
	code, err := readSecret("Código atual do autenticador: ")
	if err != nil {
		return err
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
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err = application.PromoteAdministrator(ctx, postgres.New(pool), passwords, second, security.Clock{}, username, password, second.Seal(key.Secret()), code); err != nil {
		return err
	}
	fmt.Println("Acesso administrativo ativado. Sua senha, vínculos e códigos de recuperação foram preservados.")
	fmt.Println("As sessões anteriores foram encerradas. Entre novamente e selecione Acesso administrativo.")
	fmt.Println("Aguarde o próximo código do autenticador para entrar; o código usado na ativação já foi consumido.")
	return nil
}
