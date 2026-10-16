import {
  useState,
  type ButtonHTMLAttributes,
  type InputHTMLAttributes,
  type ReactNode,
} from "react";
import {
  Check,
  Copy,
  Eye,
  EyeOff,
  Info,
  LoaderCircle,
  ShieldCheck,
} from "lucide-react";

export function Button({
  children,
  busy = false,
  variant = "primary",
  className = "",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  busy?: boolean;
  variant?: "primary" | "secondary" | "ghost";
}) {
  return (
    <button
      {...props}
      disabled={props.disabled || busy}
      className={`button button-${variant} ${className}`}
      aria-busy={busy}
    >
      {busy && <LoaderCircle className="spin" size={17} aria-hidden="true" />}
      {children}
    </button>
  );
}

export function Alert({
  children,
  tone = "error",
}: {
  children: ReactNode;
  tone?: "error" | "info" | "success";
}) {
  return (
    <div
      className={`alert alert-${tone}`}
      role={tone === "error" ? "alert" : "status"}
    >
      <Info size={18} aria-hidden="true" />
      <div>{children}</div>
    </div>
  );
}

export function Field({
  label,
  hint,
  children,
  id,
}: {
  label: string;
  hint?: string;
  children: ReactNode;
  id: string;
}) {
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      {children}
      {hint && (
        <p className="field-hint" id={`${id}-hint`}>
          {hint}
        </p>
      )}
    </div>
  );
}

export function PasswordField({
  id,
  label = "Senha",
  hint,
  ...props
}: InputHTMLAttributes<HTMLInputElement> & {
  id: string;
  label?: string;
  hint?: string;
}) {
  const [visible, setVisible] = useState(false);
  return (
    <Field id={id} label={label} hint={hint}>
      <div className="password-input">
        <input
          {...props}
          id={id}
          type={visible ? "text" : "password"}
          aria-describedby={hint ? `${id}-hint` : undefined}
        />
        <button
          type="button"
          onClick={() => setVisible(!visible)}
          aria-label={visible ? "Ocultar senha" : "Mostrar senha"}
        >
          {visible ? <EyeOff size={18} /> : <Eye size={18} />}
        </button>
      </div>
    </Field>
  );
}

export function CopyButton({
  value,
  label = "Copiar código",
}: {
  value: string;
  label?: string;
}) {
  const [copied, setCopied] = useState(false);
  const [failed, setFailed] = useState(false);
  return (
    <div className="copy-action">
      <Button
        type="button"
        variant="secondary"
        onClick={async () => {
          try {
            await navigator.clipboard.writeText(value);
            setCopied(true);
            setFailed(false);
          } catch {
            setFailed(true);
          }
        }}
      >
        {copied ? <Check size={16} /> : <Copy size={16} />}
        <span aria-live="polite">{copied ? "Copiado" : label}</span>
      </Button>
      {failed && (
        <p role="alert" className="field-hint">
          Não foi possível copiar. Selecione o código e copie manualmente.
        </p>
      )}
    </div>
  );
}

export function Loading({ text = "Carregando sua conta…" }: { text?: string }) {
  return (
    <div className="loading-state" role="status">
      <LoaderCircle className="spin" size={24} />
      <p>{text}</p>
    </div>
  );
}

export function AuthLayout({
  eyebrow,
  title,
  description,
  children,
}: {
  eyebrow: string;
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <div className="auth-layout page-width">
      <section className="auth-content">
        <span className="eyebrow">{eyebrow}</span>
        <h1>{title}</h1>
        <p className="intro">{description}</p>
        {children}
      </section>
      <aside className="auth-aside">
        <div className="aside-symbol">
          <ShieldCheck size={34} strokeWidth={1.4} />
        </div>
        <span className="eyebrow">PARTICIPAR COM CONFIANÇA</span>
        <h2>
          Uma voz identificada.
          <br />
          Uma experiência registrada.
        </h2>
        <p>
          Vincule seu perfil uma única vez e mantenha seus próximos relatos
          conectados à sua conta.
        </p>
        <div className="aside-rule" />
        <ul className="check-list">
          <li>
            <Check size={17} />
            Sem pedir a senha do Instagram
          </li>
          <li>
            <Check size={17} />
            Você escolhe o que compartilhar
          </li>
          <li>
            <Check size={17} />
            Seu acesso continua independente da rede
          </li>
        </ul>
        <span className="aside-footnote">
          O vínculo confirma o controle do perfil. Cada relato tem sua própria
          apuração.
        </span>
      </aside>
    </div>
  );
}
