import { useState, type FormEvent } from "react";
import { Link, useParams } from "react-router";
import {
  ArrowDown,
  ArrowLeft,
  ArrowRight,
  ArrowUpRight,
  Check,
  FileText,
  Link2,
  MessageSquare,
  Plus,
  Search,
  ShieldCheck,
  Users,
} from "lucide-react";
import { examples, filterExamples } from "../domain/examples";
import { Alert, Button, Field, Loading } from "../components/ui";
import { useAuth } from "../application/auth";
import { draftKey } from "../application/private-storage";

export function HomePage() {
  const [query, setQuery] = useState("");
  const [platform, setPlatform] = useState("Todas as redes");
  const items = filterExamples(query, platform);
  return (
    <>
      <section className="hero-section">
        <div className="hero page-width">
          <div className="hero-copy">
            <span className="eyebrow">
              <span className="status-dot" />
              SOBERANIA DIGITAL · MEMÓRIA COLETIVA
            </span>
            <h1>
              <span className="hero-title-line">
                Não se <em>Meta</em>
              </span>{" "}
              <span className="hero-title-line">
                no <em>Brasil.</em>
              </span>
            </h1>
            <p>
              O que acontece nas redes também diz respeito ao Brasil. Registre
              sua experiência, reúna evidências e encontre outras pessoas que
              perceberam o mesmo.
            </p>
            <div className="hero-actions">
              <Link to="/registrar" className="button button-primary">
                Preparar um relato
                <ArrowUpRight size={18} />
              </Link>
              <Link to="/como-funciona" className="text-link">
                Entenda como funciona
                <ArrowRight size={16} />
              </Link>
            </div>
            <div className="hero-footnote">
              <ShieldCheck size={17} />
              <span>Seu perfil conectado. Sua experiência documentada.</span>
            </div>
          </div>
          <div className="hero-visual" aria-hidden="true">
            <svg className="brazil-emblem" viewBox="0 0 460 380">
              <path d="M230 12 452 190 230 368 8 190Z" fill="#ffdf38" />
              <circle cx="230" cy="190" r="116" fill="#17499b" />
              <path
                d="M122 154Q230 112 340 203"
                fill="none"
                stroke="#fffdf0"
                strokeWidth="12"
              />
              <path
                d="m234 254 3 8 9 .3-7 5 2 8-7-5-7 5 2-8-7-5 9-.3Z"
                fill="#fffdf0"
              />
            </svg>
            <div className="visual-caption">
              NOSSA VOZ TEM LUGAR.
              <br />
              NOSSA MEMÓRIA TAMBÉM.
            </div>
            <div className="paper-card paper-back">
              <div className="paper-lines">
                <i />
                <i />
                <i />
              </div>
            </div>
            <div className="paper-card paper-front">
              <div className="paper-top">
                <img
                  className="paper-brand"
                  src="/favicon.svg"
                  alt=""
                  width="28"
                  height="28"
                />
                <span>UM ESPAÇO FEITO PARA O BRASIL</span>
                <ArrowUpRight size={19} />
              </div>
              <div className="paper-quote">
                Nossa voz.
                <br />
                Nossa memória.
              </div>
              <div className="paper-divider" />
              <div className="paper-bottom">
                <div className="avatar-stack">
                  <span>A</span>
                  <span>M</span>
                  <span>R</span>
                </div>
                <div>
                  Experiências que se encontram.
                  <br />
                  <strong>Evidências que permanecem.</strong>
                </div>
              </div>
            </div>
            <span className="visual-label">
              <Check size={15} />
              Do Brasil. Com contexto.
            </span>
            <span className="visual-cross">+</span>
          </div>
        </div>
      </section>
      <div className="principles-band">
        <div className="page-width">
          <p>
            <FileText size={18} />
            Registre o que aconteceu
          </p>
          <p>
            <Link2 size={18} />
            Acrescente evidências
          </p>
          <p>
            <Users size={18} />
            Conecte experiências
          </p>
          <p>
            <MessageSquare size={18} />
            Acompanhe os desdobramentos
          </p>
        </div>
      </div>
      <section className="page-width mural-section" id="mural">
        <div className="section-title-row">
          <div>
            <span className="eyebrow">
              MEMÓRIA PÚBLICA, CONSTRUÇÃO COLETIVA
            </span>
            <h2>
              Um acontecimento pode
              <br />
              ter muitas perspectivas.
            </h2>
          </div>
          <a
            href="#mural-filters"
            className="round-link"
            aria-label="Explorar exemplos"
          >
            <ArrowDown size={21} />
          </a>
        </div>
        <div className="preview-notice">
          <span className="preview-label">PRÉVIA DO MURAL</span>
          <p>
            Os cartões abaixo são <strong>exemplos ilustrativos</strong>. A
            publicação de relatos reais ainda não está disponível.
          </p>
        </div>
        <div className="mural-filters" id="mural-filters">
          <label className="search-input">
            <Search size={18} />
            <span className="sr-only">Buscar nos exemplos</span>
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Buscar por assunto ou acontecimento"
              type="search"
            />
          </label>
          <label className="filter-select">
            <span className="sr-only">Filtrar por rede</span>
            <select
              value={platform}
              onChange={(e) => setPlatform(e.target.value)}
            >
              <option>Todas as redes</option>
              <option>Instagram</option>
              <option>Facebook</option>
            </select>
          </label>
          <span className="results-count" aria-live="polite">
            {items.length} {items.length === 1 ? "exemplo" : "exemplos"}
          </span>
        </div>
        <div className="report-grid">
          {items.map((item) => (
            <Link
              className="report-card"
              to={`/relatos/${item.id}`}
              key={item.id}
            >
              <div
                className={`report-illustration illustration-${item.kind}`}
                aria-hidden="true"
              >
                <div className="mini-window">
                  <div className="mini-top">
                    <span />
                    <i />
                    <i />
                  </div>
                  {item.kind === "removed" ? (
                    <div className="mini-message">
                      <FileText size={27} strokeWidth={1.3} />
                      <span>Conteúdo indisponível</span>
                    </div>
                  ) : item.kind === "restricted" ? (
                    <div className="mini-message">
                      <ShieldCheck size={27} strokeWidth={1.3} />
                      <span>Acesso com restrições</span>
                    </div>
                  ) : (
                    <div className="mini-chart">
                      <i />
                      <i />
                      <i />
                      <i />
                      <i />
                      <i />
                      <i />
                    </div>
                  )}
                </div>
                <span className="illustration-label">EXEMPLO ILUSTRATIVO</span>
              </div>
              <div className="report-card-body">
                <div className="report-meta">
                  <span>{item.platform}</span>
                  <span>{item.category}</span>
                </div>
                <h3>{item.title}</h3>
                <p>{item.description}</p>
                <div className="report-bottom">
                  <span className="sample-status">
                    <span className="status-dot" />
                    {item.tag}
                  </span>
                  <ArrowUpRight size={20} />
                </div>
              </div>
            </Link>
          ))}
        </div>
        {items.length === 0 && (
          <div className="empty-search">
            <Search size={27} />
            <h3>Nenhum exemplo encontrado.</h3>
            <p>Tente outra palavra ou rede.</p>
            <Button
              variant="secondary"
              onClick={() => {
                setQuery("");
                setPlatform("Todas as redes");
              }}
            >
              Limpar filtros
            </Button>
          </div>
        )}
      </section>
      <section className="join-section page-width">
        <div>
          <span className="eyebrow">PARTICIPAÇÃO COM RESPONSABILIDADE</span>
          <h2>
            Sua experiência é o começo.
            <br />O contexto faz a diferença.
          </h2>
          <p>
            Crie sua conta e confirme o controle do seu Instagram. Assim, seus
            próximos relatos terão uma origem identificada.
          </p>
        </div>
        <Link to="/cadastro" className="button button-lime">
          Quero participar
          <ArrowUpRight size={18} />
        </Link>
      </section>
    </>
  );
}

export function HowItWorksPage() {
  return (
    <div className="page-width info-page">
      <span className="eyebrow">COMO FUNCIONA</span>
      <h1>
        Dar contexto.
        <br />
        Construir confiança.
      </h1>
      <p className="intro">
        Cada experiência pode ajudar a entender o que acontece nas redes. O
        caminho começa com um registro cuidadoso.
      </p>
      <div className="how-grid">
        {[
          {
            number: "01",
            title: "Prepare seu relato",
            body: "Descreva o que aconteceu. Quando possível, guarde o aviso da rede social, um link e a data aproximada.",
          },
          {
            number: "02",
            title: "Conecte seu perfil",
            body: "Crie uma conta, gere seu código e envie pelo seu Instagram. Depois da conferência da mensagem, você confirma o vínculo no site.",
          },
          {
            number: "03",
            title: "Acrescente perspectivas",
            body: "Na etapa pública, outras pessoas poderão testemunhar o mesmo caso ou acrescentar evidências, com indicação do tipo de contribuição.",
          },
        ].map((item) => (
          <section key={item.number} className="how-card">
            <span>{item.number}</span>
            <h2>{item.title}</h2>
            <p>{item.body}</p>
          </section>
        ))}
      </div>
      <div className="info-callout">
        <ShieldCheck size={30} />
        <div>
          <h2>Uma confirmação tem um significado específico.</h2>
          <p>
            Conectar um perfil confirma seu controle naquela data. Curtidas e
            relatos semelhantes não substituem evidências, nem demonstram
            sozinhos a causa de uma decisão da rede.
          </p>
        </div>
      </div>
      <Link className="button button-primary" to="/cadastro">
        Criar minha conta
        <ArrowRight size={17} />
      </Link>
    </div>
  );
}

type Draft = {
  description: string;
  category: string;
  platform: string;
  source_url: string;
};
const emptyDraft: Draft = {
  description: "",
  category: "Conteúdo removido",
  platform: "Instagram",
  source_url: "",
};
function readDraft(storageKey: string): Draft {
  try {
    const item = JSON.parse(sessionStorage.getItem(storageKey) || "null");
    return item && typeof item.description === "string"
      ? { ...emptyDraft, ...item }
      : emptyDraft;
  } catch {
    return emptyDraft;
  }
}

export function DraftPage() {
  const auth = useAuth();
  if (auth.loading) return <Loading text="Preparando seu rascunho…" />;
  const storageKey = draftKey(auth.session?.user.id);
  return <DraftEditor key={storageKey} storageKey={storageKey} />;
}

function DraftEditor({ storageKey }: { storageKey: string }) {
  const [draft, setDraft] = useState<Draft>(() => readDraft(storageKey));
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState("");
  function update(key: keyof Draft, value: string) {
    setDraft((current) => ({ ...current, [key]: value }));
    setSaved(false);
  }
  function save(event: FormEvent) {
    event.preventDefault();
    try {
      sessionStorage.setItem(storageKey, JSON.stringify(draft));
      setSaved(true);
      setError("");
    } catch {
      setError(
        "Este navegador não permitiu salvar. Mantenha uma cópia do texto antes de sair.",
      );
    }
  }
  return (
    <div className="page-width draft-layout">
      <section>
        <Link to="/" className="back-link">
          <ArrowLeft size={16} />
          Voltar ao mural
        </Link>
        <span className="eyebrow">COMECE PELO QUE VOCÊ OBSERVOU</span>
        <h1>O que aconteceu?</h1>
        <p className="intro">
          Poucos campos para organizar a experiência. Você poderá complementar o
          contexto depois.
        </p>
        <Alert tone="info">
          Prévia do formulário. Este rascunho fica somente nesta aba do
          navegador; ainda não é enviado nem publicado.
        </Alert>
        <form className="form-stack" onSubmit={save}>
          {error && <Alert>{error}</Alert>}
          <Field
            id="draft-link"
            label="Tem um link?"
            hint="Opcional. Pode ser da publicação ou do perfil envolvido."
          >
            <div className="input-icon">
              <Link2 size={18} />
              <input
                id="draft-link"
                type="url"
                maxLength={2048}
                value={draft.source_url}
                onChange={(e) => update("source_url", e.target.value)}
                placeholder="https://…"
              />
            </div>
          </Field>
          <div className="form-columns">
            <Field id="draft-category" label="Tipo de acontecimento">
              <select
                id="draft-category"
                value={draft.category}
                onChange={(e) => update("category", e.target.value)}
              >
                {[
                  "Conteúdo removido",
                  "Restrição de conta",
                  "Conta suspensa",
                  "Queda de alcance",
                  "Outro",
                ].map((item) => (
                  <option key={item}>{item}</option>
                ))}
              </select>
            </Field>
            <Field id="draft-platform" label="Em qual rede?">
              <select
                id="draft-platform"
                value={draft.platform}
                onChange={(e) => update("platform", e.target.value)}
              >
                <option>Instagram</option>
                <option>Facebook</option>
                <option>Threads</option>
                <option>Outra</option>
              </select>
            </Field>
          </div>
          <Field
            id="draft-description"
            label="Conte em poucas palavras"
            hint={`${draft.description.length}/1000 caracteres`}
          >
            <textarea
              id="draft-description"
              required
              minLength={10}
              maxLength={1000}
              rows={5}
              value={draft.description}
              onChange={(e) => update("description", e.target.value)}
              placeholder="Descreva o que você observou, quando aconteceu e se recebeu algum aviso."
              aria-describedby="draft-description-hint"
            />
          </Field>
          <div className="coming-upload">
            <Plus size={23} />
            <div>
              <strong>Prints ajudam a contar a história.</strong>
              <p>
                O envio de imagens será disponibilizado em uma próxima etapa.
              </p>
            </div>
            <span className="preview-label">EM BREVE</span>
          </div>
          {saved && (
            <Alert tone="success">
              Rascunho salvo nesta aba. Ele será mantido enquanto você prepara
              seu acesso.
            </Alert>
          )}
          <div className="button-row">
            <Button type="submit">
              Salvar rascunho
              <FileText size={17} />
            </Button>
            <Button
              type="button"
              variant="ghost"
              onClick={() => {
                try {
                  sessionStorage.removeItem(storageKey);
                } catch {}
                setDraft(emptyDraft);
                setSaved(false);
              }}
            >
              Limpar
            </Button>
          </div>
        </form>
      </section>
      <aside className="draft-aside">
        <span className="eyebrow">UM BOM REGISTRO COMEÇA ASSIM</span>
        <h2>
          O que você viu
          <br />é o ponto de partida.
        </h2>
        <ul className="check-list">
          <li>
            <Check size={17} />
            Descreva o acontecimento concreto
          </li>
          <li>
            <Check size={17} />
            Guarde avisos e justificativas
          </li>
          <li>
            <Check size={17} />
            Oculte dados pessoais dos prints
          </li>
          <li>
            <Check size={17} />
            Separe observação de interpretação
          </li>
        </ul>
        <div className="aside-rule" />
        <p>
          Seu perfil pode ser confirmado desde já, enquanto a publicação pública
          está em preparação.
        </p>
        <Link to="/cadastro" className="text-link">
          Preparar meu acesso
          <ArrowRight size={17} />
        </Link>
      </aside>
    </div>
  );
}

export function ExamplePage() {
  const { id } = useParams();
  const item = examples.find((entry) => entry.id === id);
  if (!item) return <NotFoundPage />;
  return (
    <div className="page-width narrow-page">
      <Link to="/" className="back-link">
        <ArrowLeft size={16} />
        Voltar ao mural
      </Link>
      <Alert tone="info">
        Exemplo ilustrativo de interface. Este cartão não descreve uma
        ocorrência real.
      </Alert>
      <div className="example-detail">
        <div className="report-meta">
          <span>{item.platform}</span>
          <span>{item.category}</span>
        </div>
        <h1>{item.title}</h1>
        <p className="intro">{item.description}</p>
        <div className="detail-divider" />
        <h2>Um relato reunirá contexto e evidências.</h2>
        <p>
          Aqui aparecerão a descrição do autor, documentos aprovados para
          publicação e uma linha do tempo com correções ou respostas recebidas.
        </p>
        <div className="detail-placeholder">
          <FileText size={26} />
          <span>Espaço reservado para as evidências do relato</span>
        </div>
        <h2>Outras pessoas poderão contribuir.</h2>
        <p>
          Testemunhar o mesmo caso, relatar uma experiência semelhante e
          acompanhar terão contagens distintas.
        </p>
        <Button disabled variant="secondary">
          Contribuições em breve
          <Users size={17} />
        </Button>
      </div>
    </div>
  );
}

export function NotFoundPage() {
  return (
    <div className="page-width not-found">
      <span className="eyebrow">PÁGINA NÃO ENCONTRADA</span>
      <h1>Este caminho ainda não existe.</h1>
      <p>Volte ao mural para continuar explorando.</p>
      <Link className="button button-primary" to="/">
        Ir para o início
        <ArrowRight size={17} />
      </Link>
    </div>
  );
}
