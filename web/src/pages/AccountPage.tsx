import { useState } from "react";
import { Link } from "react-router";
import {
  ArrowRight,
  BadgeCheck,
  Camera as Instagram,
  LockKeyhole,
  Plus,
  ShieldCheck,
} from "lucide-react";
import { useAuth } from "../application/auth";
import { useAction } from "../application/use-action";
import { Alert, Button } from "../components/ui";

export function AccountPage() {
  const auth = useAuth();
  const action = useAction();
  const [revoking, setRevoking] = useState(false);
  const { user, instagram_profile: profile } = auth.session!;
  return (
    <div className="page-width account-page">
      <div className="page-heading">
        <div>
          <span className="eyebrow">SEU ESPAÇO</span>
          <h1>Olá, {user.display_name || user.username}.</h1>
          <p className="intro">
            Seu acesso, seus vínculos e seus próximos passos.
          </p>
        </div>
        <Link to="/registrar" className="button button-secondary">
          <Plus size={17} />
          Preparar um relato
        </Link>
      </div>
      {action.error && <Alert>{action.error}</Alert>}
      <div className="account-grid">
        <section className="panel">
          <div className="panel-heading">
            <div className="icon-square">
              <Instagram size={24} />
            </div>
            <span className={`status-pill ${profile ? "success" : ""}`}>
              {profile ? "Conectado" : "Ainda não conectado"}
            </span>
          </div>
          <h2>Seu Instagram</h2>
          {profile ? (
            <>
              <strong className="account-handle">@{profile.handle}</strong>
              <p className="verified-line">
                <BadgeCheck size={17} />
                Controle do perfil confirmado
              </p>
              <p className="field-hint">
                Conferido em{" "}
                {new Date(profile.verified_at).toLocaleDateString("pt-BR")}. O
                vínculo está privado.
              </p>
              <button
                className="text-button subtle"
                onClick={() => setRevoking(true)}
              >
                Desvincular perfil
              </button>
              {revoking && (
                <div
                  className="inline-confirmation"
                  role="alertdialog"
                  aria-labelledby="revoke-title"
                >
                  <h3 id="revoke-title">Desvincular @{profile.handle}?</h3>
                  <p>
                    Para vincular novamente, será necessário confirmar o perfil
                    por direct.
                  </p>
                  <div className="button-row">
                    <Button
                      variant="secondary"
                      onClick={() => setRevoking(false)}
                    >
                      Cancelar
                    </Button>
                    <Button
                      busy={action.busy}
                      onClick={() =>
                        void action.run(async () => {
                          await auth.gateway.revokeProfile(profile.id);
                          await auth.refresh();
                          setRevoking(false);
                        })
                      }
                    >
                      Confirmar desvinculação
                    </Button>
                  </div>
                </div>
              )}
            </>
          ) : (
            <>
              <p>
                Confirme que o perfil é seu com uma mensagem por direct ao
                projeto.
              </p>
              <Link to="/vincular" className="button button-primary">
                Conectar meu Instagram
                <ArrowRight size={17} />
              </Link>
            </>
          )}
        </section>
        <section className="panel account-info">
          <div className="icon-square neutral">
            <LockKeyhole size={24} />
          </div>
          <h2>Seu acesso é independente.</h2>
          <p>
            Mesmo que sua conta na rede social fique indisponível, você continua
            entrando com seu usuário e senha.
          </p>
          <dl>
            <div>
              <dt>Nome de usuário</dt>
              <dd>{user.username}</dd>
            </div>
            <div>
              <dt>Conta criada em</dt>
              <dd>{new Date(user.created_at).toLocaleDateString("pt-BR")}</dd>
            </div>
          </dl>
          {user.role === "admin" && (
            <Link className="text-link" to="/admin/verificacoes">
              <ShieldCheck size={17} />
              Conferir vínculos
            </Link>
          )}
        </section>
      </div>
      <section className="next-step">
        <div>
          <span className="eyebrow">EM PREPARAÇÃO</span>
          <h2>O próximo passo é registrar.</h2>
          <p>
            Você já pode conhecer o formulário e salvar um rascunho neste
            navegador. A publicação de relatos será liberada em uma próxima
            etapa.
          </p>
        </div>
        <Link
          to="/registrar"
          className="round-link"
          aria-label="Conhecer o formulário"
        >
          <ArrowRight size={22} />
        </Link>
      </section>
    </div>
  );
}
