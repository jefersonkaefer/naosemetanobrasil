import { useEffect, useState, type ReactNode } from "react";
import {
  Link,
  NavLink,
  Navigate,
  Route,
  Routes,
  useLocation,
  useNavigate,
} from "react-router";
import { ArrowUpRight, LogOut, Menu, ShieldCheck, X } from "lucide-react";
import { useAuth } from "./application/auth";
import { useAction } from "./application/use-action";
import { Alert, Button, Loading } from "./components/ui";
import { LoginPage, RecoverPage, RegisterPage } from "./pages/AuthPages";
import { AccountPage } from "./pages/AccountPage";
import { VerificationPage } from "./pages/VerificationPage";
import { AdminPage } from "./pages/AdminPage";
import {
  DraftPage,
  ExamplePage,
  HomePage,
  HowItWorksPage,
  NotFoundPage,
} from "./pages/PublicPages";

function Protected({
  children,
  admin = false,
}: {
  children: ReactNode;
  admin?: boolean;
}) {
  const auth = useAuth();
  const location = useLocation();
  if (auth.loading)
    return (
      <Loading text="Consultando sua conta. A primeira conexão pode levar um pouco…" />
    );
  if (!auth.session && auth.connectionError)
    return (
      <div className="page-width narrow-page">
        <Alert>
          Não conseguimos consultar sua conta. Tente conectar novamente pelo
          aviso acima.
        </Alert>
      </div>
    );
  if (!auth.session)
    return (
      <Navigate to="/entrar" replace state={{ from: location.pathname }} />
    );
  if (admin && auth.session.user.role !== "admin")
    return (
      <div className="page-width narrow-page">
        <Alert>Esta área está disponível apenas para administradores.</Alert>
        <Link className="button button-secondary" to="/conta">
          Voltar à minha conta
        </Link>
      </div>
    );
  return children;
}

export function App() {
  const auth = useAuth();
  const action = useAction();
  const location = useLocation();
  const navigate = useNavigate();
  const [menu, setMenu] = useState(false);
  const [retrying, setRetrying] = useState(false);
  useEffect(() => {
    setMenu(false);
    window.scrollTo(0, 0);
    const titles: Record<string, string> = {
      "/": "Soberania digital e memória coletiva",
      "/cadastro": "Criar conta",
      "/entrar": "Entrar",
      "/recuperar": "Recuperar acesso",
      "/conta": "Minha conta",
      "/vincular": "Vincular Instagram",
      "/registrar": "Preparar relato",
      "/como-funciona": "Como funciona",
      "/admin/verificacoes": "Conferir vínculos",
    };
    document.title = `${titles[location.pathname] || "Relatos"} — Não se Meta no Brasil`;
  }, [location.pathname]);
  return (
    <div className="app-shell">
      <a className="skip-link" href="#main">
        Pular para o conteúdo
      </a>
      <header className="site-header">
        <div className="page-width header-inner">
          <Link
            className="brand"
            to="/"
            aria-label="Não se Meta no Brasil, início"
          >
            <img
              className="brand-mark"
              src="/favicon.svg"
              alt=""
              width="36"
              height="36"
            />
            <span className="brand-wordmark">
              <span>
                não se <strong>meta</strong>
              </span>
              <span>
                no <strong>brasil.</strong>
              </span>
            </span>
          </Link>
          <nav className="desktop-nav" aria-label="Navegação principal">
            <NavLink to="/" end>
              Explorar
            </NavLink>
            <NavLink to="/como-funciona">Como funciona</NavLink>
          </nav>
          <div className="header-actions">
            {auth.session ? (
              <>
                <Link to="/conta" className="account-link">
                  <span className="user-avatar">
                    {(
                      auth.session.user.display_name ||
                      auth.session.user.username
                    )
                      .charAt(0)
                      .toUpperCase()}
                  </span>
                  <span>Minha conta</span>
                </Link>
                <button
                  className="icon-button logout-button"
                  disabled={action.busy}
                  aria-label="Sair da conta"
                  onClick={() =>
                    void action.run(async () => {
                      await auth.logout();
                      navigate("/");
                    })
                  }
                >
                  <LogOut size={18} />
                </button>
              </>
            ) : (
              <>
                <Link className="login-link" to="/entrar">
                  Entrar
                </Link>
                <Link
                  to="/cadastro"
                  className="button button-small button-primary"
                >
                  Criar conta
                  <ArrowUpRight size={15} />
                </Link>
              </>
            )}
            <button
              className="icon-button mobile-menu-button"
              aria-label={menu ? "Fechar menu" : "Abrir menu"}
              aria-expanded={menu}
              aria-controls="mobile-menu"
              onClick={() => setMenu(!menu)}
            >
              {menu ? <X size={22} /> : <Menu size={22} />}
            </button>
          </div>
        </div>
        {menu && (
          <nav
            id="mobile-menu"
            className="mobile-nav"
            aria-label="Navegação móvel"
          >
            <Link to="/">Explorar</Link>
            <Link to="/como-funciona">Como funciona</Link>
            <Link to="/registrar">Preparar relato</Link>
            {auth.session?.user.role === "admin" && (
              <Link to="/admin/verificacoes">Conferir vínculos</Link>
            )}
          </nav>
        )}
      </header>
      {auth.connectionError && (
        <div className="connection-banner" role="status">
          <span>
            Não conseguimos conectar à plataforma. Você pode explorar as prévias
            e tentar novamente.
          </span>
          <Button
            variant="ghost"
            busy={retrying}
            onClick={async () => {
              setRetrying(true);
              try {
                await auth.refresh();
              } catch {
              } finally {
                setRetrying(false);
              }
            }}
          >
            Tentar conectar
          </Button>
        </div>
      )}
      {action.error && (
        <div className="page-width">
          <Alert>{action.error}</Alert>
        </div>
      )}
      <main id="main" tabIndex={-1}>
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="/como-funciona" element={<HowItWorksPage />} />
          <Route path="/registrar" element={<DraftPage />} />
          <Route path="/relatos/:id" element={<ExamplePage />} />
          <Route path="/entrar" element={<LoginPage />} />
          <Route path="/cadastro" element={<RegisterPage />} />
          <Route path="/recuperar" element={<RecoverPage />} />
          <Route
            path="/conta"
            element={
              <Protected>
                <AccountPage />
              </Protected>
            }
          />
          <Route
            path="/vincular"
            element={
              <Protected>
                <VerificationPage />
              </Protected>
            }
          />
          <Route
            path="/admin/verificacoes"
            element={
              <Protected admin>
                <AdminPage />
              </Protected>
            }
          />
          <Route path="*" element={<NotFoundPage />} />
        </Routes>
      </main>
      <footer className="site-footer page-width">
        <Link className="footer-brand" to="/">
          não se meta no brasil.
        </Link>
        <p>Projeto independente · versão inicial</p>
        <a
          href="https://www.instagram.com/naosemetanobrasil/"
          target="_blank"
          rel="noreferrer"
        >
          @naosemetanobrasil
          <ArrowUpRight size={15} />
        </a>
        <Link to="/como-funciona">
          <ShieldCheck size={15} />
          Transparência começa com contexto
        </Link>
      </footer>
    </div>
  );
}
