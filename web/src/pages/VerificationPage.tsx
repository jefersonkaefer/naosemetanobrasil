import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type FormEvent,
} from "react";
import { Link } from "react-router";
import {
  ArrowLeft,
  ArrowRight,
  Check,
  CheckCircle2,
  Clock3,
  ExternalLink,
  Camera as Instagram,
  RotateCw,
  ShieldCheck,
} from "lucide-react";
import { useAuth } from "../application/auth";
import { useAction } from "../application/use-action";
import {
  activeStatus,
  messageFor,
  PlatformError,
  type Challenge,
  type IssuedChallenge,
} from "../domain/platform";
import { Alert, Button, CopyButton, Field, Loading } from "../components/ui";

function readPending(key: string): string | null {
  try {
    return sessionStorage.getItem(key);
  } catch {
    return null;
  }
}
function rememberPending(key: string, id: string | null) {
  try {
    if (id) sessionStorage.setItem(key, id);
    else sessionStorage.removeItem(key);
  } catch {
    /* Linking still works without browser storage. */
  }
}

export function VerificationPage() {
  const auth = useAuth();
  const action = useAction();
  const user = auth.session!.user;
  const key = `nsm:challenge:${user.id}`;
  const [id, setID] = useState(() => readPending(key));
  const [challenge, setChallenge] = useState<Challenge | null>(null);
  const [issued, setIssued] = useState<IssuedChallenge | null>(null);
  const [handle, setHandle] = useState("");
  const [checking, setChecking] = useState(Boolean(id));
  const [checkError, setCheckError] = useState("");
  const [newRequest, setNewRequest] = useState(false);
  const [, tick] = useState(0);
  const latestCheck = useRef(0);

  const check = useCallback(
    async (signal?: AbortSignal) => {
      if (!id) return;
      const request = ++latestCheck.current;
      setChecking(true);
      setCheckError("");
      try {
        const result = await auth.gateway.challenge(id, signal);
        if (signal?.aborted || request !== latestCheck.current) return;
        setChallenge(result);
        setHandle(result.expected_handle);
        if (result.status === "verified") {
          rememberPending(key, null);
          await auth.refresh();
        }
      } catch (error) {
        if (signal?.aborted || request !== latestCheck.current) return;
        if (
          error instanceof PlatformError &&
          [400, 404].includes(error.status)
        ) {
          rememberPending(key, null);
          setID(null);
          setChallenge(null);
        } else {
          setCheckError(messageFor(error));
          if (error instanceof PlatformError && error.status === 401)
            void auth.refresh().catch(() => {});
        }
      } finally {
        if (!signal?.aborted && request === latestCheck.current)
          setChecking(false);
      }
    },
    [id, auth.gateway, auth.refresh, key],
  );
  useEffect(() => {
    const controller = new AbortController();
    void check(controller.signal);
    return () => controller.abort();
  }, [check]);
  useEffect(() => {
    const onFocus = () => {
      if (id) void check();
    };
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, [id, check]);
  // Only updates the local expiry display; no keep-alive requests or background polling.
  useEffect(() => {
    const timer = setInterval(() => tick((value) => value + 1), 30_000);
    return () => clearInterval(timer);
  }, []);

  const profile = auth.session!.instagram_profile;
  if (profile)
    return (
      <div className="page-width narrow-page">
        <Link to="/conta" className="back-link">
          <ArrowLeft size={16} />
          Minha conta
        </Link>
        <div className="connected-card">
          <div className="success-icon">
            <ShieldCheck size={34} />
          </div>
          <span className="eyebrow">VÍNCULO CONCLUÍDO</span>
          <h1>Seu Instagram está conectado.</h1>
          <a
            href={`https://www.instagram.com/${encodeURIComponent(profile.handle)}/`}
            target="_blank"
            rel="noreferrer"
            className="profile-handle"
          >
            @{profile.handle}
            <ExternalLink size={18} />
          </a>
          <p>
            Controle do perfil confirmado em{" "}
            {new Date(profile.verified_at).toLocaleDateString("pt-BR")}. Seu
            vínculo permanece privado nesta etapa.
          </p>
          <Link to="/conta" className="button button-primary">
            Ir para minha conta
            <ArrowRight size={17} />
          </Link>
        </div>
      </div>
    );

  const status = challenge ? activeStatus(challenge) : null;
  const automatic =
    (issued?.verification_mode || challenge?.verification_mode) === "webhook";
  const terminal =
    status && ["expired", "rejected", "cancelled"].includes(status);
  const showForm = (!id || newRequest || terminal) && !checking;
  const step =
    status === "awaiting_confirmation"
      ? 3
      : challenge?.sent_at
        ? 2
        : challenge
          ? 1
          : 0;

  function submit(event: FormEvent) {
    event.preventDefault();
    void action.run(async () => {
      const result = await auth.gateway.issueChallenge(handle.trim());
      rememberPending(key, result.challenge.id);
      setIssued(result);
      setChallenge(result.challenge);
      setID(result.challenge.id);
      setNewRequest(false);
    });
  }
  return (
    <div className="page-width verification-layout">
      <section>
        <Link to="/conta" className="back-link">
          <ArrowLeft size={16} />
          Minha conta
        </Link>
        <span className="eyebrow">SEU PERFIL, SUA PARTICIPAÇÃO</span>
        <h1>Conecte seu Instagram.</h1>
        <p className="intro">
          Uma confirmação simples, feita uma única vez. Sua conta pode continuar
          pessoal.
        </p>
        <ol className="verification-steps">
          {[
            "Seu perfil",
            "Enviar código",
            "Conferência",
            "Confirmar vínculo",
          ].map((label, index) => (
            <li key={label} className={index <= step ? "active" : ""}>
              <span>{index < step ? <Check size={15} /> : index + 1}</span>
              <p>{label}</p>
            </li>
          ))}
        </ol>
        {action.error && <Alert>{action.error}</Alert>}
        {checkError && (
          <Alert>
            {checkError}{" "}
            <button className="text-button" onClick={() => void check()}>
              Tentar novamente
            </button>
          </Alert>
        )}
        {checking && !challenge && <Loading text="Consultando seu pedido…" />}
        {terminal && (
          <Alert tone="info">
            {status === "expired"
              ? "O prazo deste código terminou. Gere outro para continuar."
              : status === "rejected"
                ? "Não foi possível confirmar este pedido. Confira seu perfil e tente novamente."
                : "Este código foi cancelado. Gere outro para continuar."}
          </Alert>
        )}
        {showForm && (
          <form className="form-stack verification-form" onSubmit={submit}>
            <Field
              id="instagram-handle"
              label="Qual é seu perfil no Instagram?"
              hint="Informe o @ da conta de onde você enviará a mensagem."
            >
              <div className="handle-input">
                <span aria-hidden="true">@</span>
                <input
                  id="instagram-handle"
                  autoComplete="off"
                  autoCapitalize="none"
                  spellCheck={false}
                  required
                  maxLength={31}
                  value={handle}
                  onChange={(e) => setHandle(e.target.value.replace(/^@/, ""))}
                  placeholder="seu.perfil"
                  aria-describedby="instagram-handle-hint"
                />
              </div>
            </Field>
            {newRequest && (
              <p className="field-hint">
                O código anterior será cancelado ao gerar um novo.
              </p>
            )}
            <Button type="submit" busy={action.busy}>
              Gerar meu código
              <ArrowRight size={17} />
            </Button>
            {newRequest && (
              <Button
                type="button"
                variant="ghost"
                onClick={() => setNewRequest(false)}
              >
                Manter pedido atual
              </Button>
            )}
          </form>
        )}
        {!showForm && challenge && !terminal && (
          <div className="verification-card">
            {status === "awaiting_confirmation" ? (
              <>
                <span className="status-pill success">
                  <CheckCircle2 size={15} />
                  Conferência concluída
                </span>
                <h2>Este é seu perfil?</h2>
                <p>
                  {challenge.verification_method === "instagram_webhook"
                    ? "Recebemos seu código e conferimos o remetente automaticamente."
                    : "A mensagem foi conferida."}{" "}
                  Confirme o perfil abaixo para concluir.
                </p>
                <strong className="observed-handle">
                  @{challenge.observed_handle}
                </strong>
                <Button
                  busy={action.busy}
                  onClick={() =>
                    void action.run(async () => {
                      await auth.gateway.confirm(challenge.id);
                      rememberPending(key, null);
                      setIssued(null);
                      await auth.refresh();
                    })
                  }
                >
                  Confirmar @{challenge.observed_handle}
                  <Check size={17} />
                </Button>
              </>
            ) : (
              <>
                <span className="status-pill">
                  <Clock3 size={15} />
                  {challenge.sent_at
                    ? "Aguardando conferência"
                    : "Código gerado"}
                </span>
                <h2>
                  {challenge.sent_at
                    ? "Mensagem enviada. Agora é aguardar."
                    : "Envie seu código por direct."}
                </h2>
                {issued ? (
                  <>
                    <p>
                      Use o perfil <strong>@{challenge.expected_handle}</strong>{" "}
                      para enviar este código a{" "}
                      <strong>@{issued.official_handle}</strong>.
                    </p>
                    <div className="verification-code">
                      <code>{issued.code}</code>
                      <CopyButton value={issued.code} />
                    </div>
                    <a
                      className="button button-primary"
                      href={`https://ig.me/m/${encodeURIComponent(issued.official_handle)}`}
                      target="_blank"
                      rel="noreferrer"
                    >
                      <Instagram size={18} />
                      Abrir conversa no Instagram
                      <ExternalLink size={16} />
                    </a>
                  </>
                ) : (
                  <p>
                    O código foi exibido uma única vez. Se você ainda não o
                    enviou e não tem uma cópia, gere outro código.
                  </p>
                )}
                <p className="expiry-note">
                  Válido até{" "}
                  {new Date(challenge.expires_at).toLocaleString("pt-BR", {
                    day: "2-digit",
                    month: "2-digit",
                    hour: "2-digit",
                    minute: "2-digit",
                  })}
                  .
                </p>
                {!challenge.sent_at && (
                  <Button
                    variant="secondary"
                    busy={action.busy}
                    onClick={() =>
                      void action.run(async () => {
                        await auth.gateway.markSent(challenge.id);
                        await check();
                      })
                    }
                  >
                    Já enviei o código
                    <Check size={17} />
                  </Button>
                )}
                {challenge.sent_at && (
                  <p className="small-note">
                    <Clock3 size={17} />
                    {automatic
                      ? "Assim que recebermos sua mensagem pelo Instagram, o código e o remetente serão conferidos automaticamente. Atualize o status ao voltar."
                      : "A conferência é manual neste ambiente. Atualize o status para saber quando estiver pronta."}
                  </p>
                )}
              </>
            )}
            <div className="verification-actions">
              <Button
                variant="ghost"
                busy={checking}
                onClick={() => void check()}
              >
                <RotateCw size={16} />
                Atualizar status
              </Button>
              <button
                className="text-button subtle"
                disabled={action.busy}
                onClick={() => setNewRequest(true)}
              >
                Gerar outro código
              </button>
            </div>
          </div>
        )}
      </section>
      <aside className="verification-aside">
        <div className="aside-symbol light">
          <Instagram size={25} />
        </div>
        <h2>O que estamos confirmando?</h2>
        <p>
          Que você controla o perfil informado. Sua senha do Instagram fica com
          você.
        </p>
        <ul className="check-list">
          <li>
            <Check size={17} />
            Sem mudar sua bio
          </li>
          <li>
            <Check size={17} />
            Sem precisar de conta profissional
          </li>
          <li>
            <Check size={17} />
            Sem mensagem enviada automaticamente
          </li>
        </ul>
        <div className="aside-rule" />
        <p className="field-hint">
          O vínculo com o perfil e a apuração de cada acontecimento são
          informações distintas.
        </p>
      </aside>
    </div>
  );
}
