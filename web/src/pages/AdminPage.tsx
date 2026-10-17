import { useCallback, useEffect, useState, type FormEvent } from "react";
import { CheckCircle2, RotateCw, ShieldCheck } from "lucide-react";
import { useAuth } from "../application/auth";
import { useAction } from "../application/use-action";
import { messageFor, type QueueItem } from "../domain/platform";
import { Alert, Button, Field } from "../components/ui";

export function AdminPage() {
  const auth = useAuth();
  const action = useAction();
  const [items, setItems] = useState<QueueItem[]>([]);
  const [loadError, setLoadError] = useState("");
  const [loaded, setLoaded] = useState(false);
  const [code, setCode] = useState("");
  const [handle, setHandle] = useState("");
  const [reference, setReference] = useState("");
  const [checked, setChecked] = useState(false);
  const [success, setSuccess] = useState("");
  const load = useCallback(async () => {
    try {
      const result = await auth.gateway.queue();
      setItems(result.items);
      setLoadError("");
      setLoaded(true);
    } catch (error) {
      setLoadError(messageFor(error));
    }
  }, [auth.gateway]);
  useEffect(() => {
    void load();
  }, [load]);
  function submit(event: FormEvent) {
    event.preventDefault();
    void action.run(async () => {
      if (!checked) return;
      const result = await auth.gateway.observe({
        code: code.trim(),
        observed_handle: handle.trim(),
        reference: reference.trim(),
      });
      setSuccess(
        `Mensagem de @${result.observed_handle} conferida. A confirmação final cabe à pessoa.`,
      );
      setCode("");
      setHandle("");
      setReference("");
      setChecked(false);
      await load();
    });
  }
  return (
    <div className="page-width account-page">
      <div className="page-heading">
        <div>
          <span className="eyebrow">ÁREA ADMINISTRATIVA</span>
          <h1>Conferência de vínculos.</h1>
          <p className="intro">
            Confira a mensagem na caixa oficial antes de registrar a observação.
          </p>
        </div>
        <ShieldCheck size={35} className="muted-icon" />
      </div>
      <div className="admin-grid">
        <section className="panel">
          <h2>Registrar mensagem conferida</h2>
          <p className="field-hint">
            Você não pode conferir o vínculo da sua própria conta. Outro
            administrador precisa conferir sua mensagem no Instagram.
          </p>
          <form className="form-stack" onSubmit={submit}>
            {action.error && <Alert>{action.error}</Alert>}
            {success && (
              <Alert tone="success">
                <CheckCircle2 size={16} />
                {success}
              </Alert>
            )}
            <Field id="admin-code" label="Código recebido no direct">
              <input
                id="admin-code"
                required
                autoComplete="off"
                spellCheck={false}
                maxLength={100}
                value={code}
                onChange={(e) => setCode(e.target.value)}
                placeholder="NSM-…"
              />
            </Field>
            <Field id="admin-handle" label="Perfil real do remetente">
              <input
                id="admin-handle"
                required
                autoComplete="off"
                spellCheck={false}
                maxLength={31}
                value={handle}
                onChange={(e) => setHandle(e.target.value)}
                placeholder="@perfil"
              />
            </Field>
            <Field
              id="admin-reference"
              label="Referência da conferência"
              hint="Uma referência curta para localizar sua conferência. Não copie a conversa inteira."
            >
              <input
                id="admin-reference"
                required
                maxLength={200}
                value={reference}
                onChange={(e) => setReference(e.target.value)}
                aria-describedby="admin-reference-hint"
              />
            </Field>
            <label className="checkbox-label">
              <input
                type="checkbox"
                required
                checked={checked}
                onChange={(e) => setChecked(e.target.checked)}
              />
              Conferi a mensagem e o remetente na caixa oficial do Instagram.
            </label>
            <Button type="submit" busy={action.busy} disabled={!checked}>
              Registrar conferência
            </Button>
          </form>
        </section>
        <section className="panel">
          <div className="section-title-row">
            <h2>Pedidos pendentes</h2>
            <Button
              variant="ghost"
              onClick={() => void load()}
              aria-label="Atualizar pedidos"
            >
              <RotateCw size={17} />
            </Button>
          </div>
          {loadError && <Alert>{loadError}</Alert>}
          {loaded && items.length === 0 && (
            <p className="empty-text">Nenhum pedido pendente no momento.</p>
          )}
          <ul className="pending-list">
            {items.map((item) => (
              <li key={item.id}>
                <strong>@{item.expected_handle}</strong>
                {item.user_id === auth.session?.user.id && (
                  <small>Seu pedido — precisa de outro administrador.</small>
                )}
                <span className="status-pill">
                  {item.status === "awaiting_confirmation"
                    ? "Aguardando a pessoa"
                    : "Aguardando conferência"}
                </span>
                <small>
                  Até {new Date(item.expires_at).toLocaleString("pt-BR")}
                </small>
              </li>
            ))}
          </ul>
        </section>
      </div>
    </div>
  );
}
