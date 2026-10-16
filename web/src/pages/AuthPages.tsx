import { useState, type FormEvent } from "react";
import { Link, Navigate, useLocation, useNavigate } from "react-router";
import { ArrowRight, CheckCircle2, ShieldCheck } from "lucide-react";
import { useAuth } from "../application/auth";
import { useAction } from "../application/use-action";
import type { Registration } from "../domain/platform";
import {
  Alert,
  AuthLayout,
  Button,
  Field,
  PasswordField,
} from "../components/ui";
import { RecoveryCodes } from "../components/RecoveryCodes";

export function LoginPage() {
  const auth = useAuth();
  const action = useAction();
  const navigate = useNavigate();
  const location = useLocation();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [admin, setAdmin] = useState(false);
  const [totp, setTotp] = useState("");
  if (auth.session) return <Navigate to="/conta" replace />;
  function submit(event: FormEvent) {
    event.preventDefault();
    void action.run(async () => {
      await auth.login({
        username: username.trim(),
        password,
        ...(admin ? { totp } : {}),
      });
      setPassword("");
      setTotp("");
      const from = (location.state as { from?: string } | null)?.from;
      navigate(
        from && ["/conta", "/vincular", "/admin/verificacoes"].includes(from)
          ? from
          : "/conta",
        { replace: true },
      );
    });
  }
  return (
    <AuthLayout
      eyebrow="BEM-VINDO DE VOLTA"
      title="Sua voz tem lugar aqui."
      description="Entre para acompanhar seu perfil e preparar seus próximos relatos."
    >
      <form onSubmit={submit} className="form-stack">
        {action.error && <Alert>{action.error}</Alert>}
        <Field id="login-username" label="Nome de usuário">
          <input
            id="login-username"
            autoComplete="username"
            required
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            maxLength={100}
            autoCapitalize="none"
            spellCheck={false}
            placeholder="Seu nome de usuário"
          />
        </Field>
        <PasswordField
          id="login-password"
          required
          autoComplete="current-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          maxLength={128}
          placeholder="Sua senha"
        />
        <div className="form-inline">
          <Link to="/recuperar" className="text-link">
            Esqueci minha senha
          </Link>
          <button
            className="text-button subtle"
            type="button"
            onClick={() => setAdmin(!admin)}
            aria-expanded={admin}
          >
            <ShieldCheck size={14} />
            Acesso administrativo
          </button>
        </div>
        {admin && (
          <Field
            id="login-totp"
            label="Código do autenticador"
            hint="Informe os seis números do seu aplicativo autenticador."
          >
            <input
              id="login-totp"
              inputMode="numeric"
              autoComplete="one-time-code"
              pattern="[0-9]{6}"
              maxLength={6}
              required
              value={totp}
              onChange={(e) => setTotp(e.target.value.replace(/\D/g, ""))}
              placeholder="000000"
            />
          </Field>
        )}
        <Button type="submit" busy={action.busy} className="full-width">
          Entrar na minha conta
          <ArrowRight size={17} />
        </Button>
      </form>
      <p className="auth-switch">
        Ainda não tem conta? <Link to="/cadastro">Criar minha conta</Link>
      </p>
    </AuthLayout>
  );
}

export function RegisterPage() {
  const auth = useAuth();
  const action = useAction();
  const navigate = useNavigate();
  const [username, setUsername] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [created, setCreated] = useState<Registration | null>(null);
  const [loginFailed, setLoginFailed] = useState(false);
  if (auth.session && !created) return <Navigate to="/conta" replace />;
  function submit(event: FormEvent) {
    event.preventDefault();
    void action.run(async () => {
      if (new TextEncoder().encode(password).length > 128) {
        action.setError(
          "Use uma senha mais curta, com pelo menos 12 caracteres.",
        );
        return;
      }
      const result = await auth.gateway.register({
        username: username.trim(),
        display_name: displayName.trim(),
        password,
      });
      setCreated(result);
      setPassword("");
      try {
        await auth.login({ username: result.user.username, password });
      } catch {
        setLoginFailed(true);
      }
    });
  }
  return (
    <AuthLayout
      eyebrow={created ? "CONTA CRIADA" : "COMECE A PARTICIPAR"}
      title={
        created ? "Seu acesso está pronto." : "Sua experiência merece registro."
      }
      description={
        created
          ? "Antes de continuar, guarde os códigos para recuperar sua conta se esquecer a senha."
          : "Crie sua conta. Depois, confirme seu Instagram uma única vez."
      }
    >
      {created ? (
        <>
          {loginFailed && (
            <Alert tone="info">
              Sua conta foi criada. Guarde os códigos e entre com sua senha para
              continuar.
            </Alert>
          )}
          <RecoveryCodes
            codes={created.recovery_codes}
            busy={action.busy}
            onContinue={() =>
              navigate(auth.session ? "/vincular" : "/entrar", {
                replace: true,
              })
            }
          />
        </>
      ) : (
        <>
          <form onSubmit={submit} className="form-stack">
            {action.error && <Alert>{action.error}</Alert>}
            <Field
              id="register-name"
              label="Como você quer ser chamado?"
              hint="Opcional. Você pode usar um nome de preferência."
            >
              <input
                id="register-name"
                autoComplete="nickname"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                maxLength={80}
                placeholder="Seu nome de preferência"
              />
            </Field>
            <Field
              id="register-username"
              label="Nome de usuário"
              hint="De 3 a 32 letras, números ou sublinhado. Este é seu acesso à plataforma."
            >
              <input
                id="register-username"
                autoComplete="username"
                required
                minLength={3}
                maxLength={32}
                pattern="[A-Za-z0-9_]{3,32}"
                autoCapitalize="none"
                spellCheck={false}
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="Ex.: ana_silva"
                aria-describedby="register-username-hint"
              />
            </Field>
            <PasswordField
              id="register-password"
              hint="Use pelo menos 12 caracteres."
              autoComplete="new-password"
              required
              minLength={12}
              maxLength={128}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Crie uma senha segura"
            />
            <div className="small-note">
              <ShieldCheck size={17} />
              <p>Não pedimos CPF, telefone ou a senha da sua rede social.</p>
            </div>
            <Button type="submit" busy={action.busy} className="full-width">
              Criar minha conta
              <ArrowRight size={17} />
            </Button>
          </form>
          <p className="auth-switch">
            Já tem uma conta? <Link to="/entrar">Entrar</Link>
          </p>
        </>
      )}
    </AuthLayout>
  );
}

export function RecoverPage() {
  const auth = useAuth();
  const action = useAction();
  const [username, setUsername] = useState("");
  const [code, setCode] = useState("");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [done, setDone] = useState(false);
  function submit(event: FormEvent) {
    event.preventDefault();
    if (password !== confirmation) {
      action.setError("As senhas precisam ser iguais.");
      return;
    }
    void action.run(async () => {
      await auth.gateway.recover({
        username: username.trim(),
        recovery_code: code.trim(),
        new_password: password,
      });
      setCode("");
      setPassword("");
      setConfirmation("");
      setDone(true);
      await auth.refresh().catch(() => {});
    });
  }
  return (
    <AuthLayout
      eyebrow="RECUPERAR ACESSO"
      title={
        done ? "Você pode entrar novamente." : "Vamos recuperar seu acesso."
      }
      description={
        done
          ? "Sua senha foi atualizada e as sessões anteriores dessa conta foram encerradas."
          : "Use um dos códigos que você guardou ao criar sua conta."
      }
    >
      {done ? (
        <div className="success-panel">
          <CheckCircle2 size={34} />
          <h2>Senha atualizada</h2>
          <p>
            O código usado foi consumido. Guarde os restantes para outra
            necessidade.
          </p>
          <Link className="button button-primary full-width" to="/entrar">
            Entrar com a nova senha
            <ArrowRight size={17} />
          </Link>
        </div>
      ) : (
        <form onSubmit={submit} className="form-stack">
          {action.error && <Alert>{action.error}</Alert>}
          <Field id="recover-username" label="Nome de usuário">
            <input
              id="recover-username"
              autoComplete="username"
              required
              maxLength={32}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoCapitalize="none"
              spellCheck={false}
            />
          </Field>
          <Field
            id="recover-code"
            label="Código de recuperação"
            hint="É um código que começa com NSR-, fornecido no cadastro."
          >
            <input
              id="recover-code"
              required
              value={code}
              onChange={(e) => setCode(e.target.value)}
              maxLength={100}
              autoComplete="off"
              autoCapitalize="none"
              spellCheck={false}
              placeholder="NSR-…"
              aria-describedby="recover-code-hint"
            />
          </Field>
          <PasswordField
            id="recover-password"
            label="Nova senha"
            hint="Use pelo menos 12 caracteres."
            required
            minLength={12}
            maxLength={128}
            autoComplete="new-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
          <PasswordField
            id="recover-confirmation"
            label="Repita a nova senha"
            required
            minLength={12}
            maxLength={128}
            autoComplete="new-password"
            value={confirmation}
            onChange={(e) => setConfirmation(e.target.value)}
          />
          <Button type="submit" busy={action.busy} className="full-width">
            Atualizar minha senha
            <ArrowRight size={17} />
          </Button>
          <p className="field-hint">
            Sem a senha e sem um código de recuperação, a recuperação automática
            não está disponível.
          </p>
          <Link to="/entrar" className="text-link">
            Voltar para entrar
          </Link>
        </form>
      )}
    </AuthLayout>
  );
}
