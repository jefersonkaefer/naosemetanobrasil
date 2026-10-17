# Não se Meta no Brasil — Go + React

Domínio registrado pelo responsável: `naosemetanobrasil.online`. Instagram oficial criado: `@naosemetanobrasil`. A prévia local continua em localhost; a configuração de DNS e a publicação ainda precisam ser realizadas. Em produção, definir `PUBLIC_BASE_URL=https://naosemetanobrasil.online` depois de conectar o domínio à hospedagem e usar `INSTAGRAM_OFFICIAL_HANDLE=naosemetanobrasil` para receber os códigos de vínculo por mensagem direta.

Autenticação local e vínculo do Instagram por direct, com backend Go, PostgreSQL e frontend React/TypeScript. A pessoa envia o código pelo Instagram. No modo `webhook`, o backend confere automaticamente a mensagem assinada e o remetente pela API da Meta; no modo `manual`, um administrador confere a caixa de entrada. Em ambos, a pessoa confirma o vínculo no site.

O frontend tem cadastro, recuperação, minha conta, vínculo e conferência administrativa com integração real. O mural e o formulário de relato são prévias identificadas: rascunhos ficam somente na aba do navegador e ainda não são publicados.

## Arquitetura hexagonal e SOLID

```text
HTTP (porta de entrada)
        |
        v
Casos de uso — application
        |              |
        v              v
Domínio puro       Portas de saída
                       |
              PostgreSQL / segurança / relógio
```

As dependências apontam para dentro. O domínio conhece apenas a biblioteca padrão. Os casos de uso dependem de interfaces pequenas, definidas conforme sua necessidade. HTTP, PostgreSQL e criptografia são adaptadores; a composição concreta fica em `cmd/`. Os adaptadores não importam uns aos outros.

- **S:** regras de transição, casos de uso, persistência e transporte têm responsabilidades separadas.
- **O:** novos adaptadores implementam as portas sem alterar o domínio.
- **L:** adaptadores devem preservar os contratos de erro, atomicidade e autorização das portas.
- **I:** contas, sessões, verificação, relógio, limitação de uso e segurança têm contratos distintos.
- **D:** casos de uso dependem dessas abstrações, sem conhecer driver SQL ou framework HTTP.

Os testes de arquitetura impedem importações de infraestrutura no núcleo. Transações ficam no adaptador PostgreSQL, que chama as regras de domínio enquanto mantém os registros bloqueados. Interfaces não existem apenas para envolver cada struct: são usadas nas fronteiras que precisam de substituição ou isolamento.

## O que está funcionando

- Cadastro com Argon2id e dez códigos de recuperação de uso único.
- Login por cookie opaco, logout, recuperação e revogação de sessões anteriores.
- Segundo fator TOTP para administradores, cifrado em repouso; login rejeita reutilização do mesmo código.
- Proteção de origem, CSRF, JSON estrito, limite de corpo e limitação persistente de tentativas.
- Emissão de código para direct, indicação de envio, conferência administrativa e confirmação final pelo dono.
- Expiração de 24 horas, cancelamento do desafio anterior, vínculo único, auditoria e desvinculação.
- Migrações versionadas com checksum e lock transacional.
- Endpoints de saúde, logs sem credenciais e encerramento controlado.
- Frontend responsivo, servido pela API Go no mesmo endereço, com fontes locais e política de segurança de conteúdo.
- Códigos de recuperação e de direct não são persistidos no armazenamento do navegador; a retomada de vínculo guarda apenas o ID do desafio, por conta e por aba.

## Frontend

O frontend está em `web/`, com React, TypeScript e Vite. Requer Node 24.19+; a versão de referência está em `web/.nvmrc`. A imagem Docker já faz o build completo; em produção não há processo Node adicional.

```sh
rtk proxy npm --prefix web ci
rtk proxy npm --prefix web run build
```

Depois do build, a API Go serve `web/dist` por padrão. Para executar com outro diretório, configurar `FRONTEND_DIR`. Em produção, a inicialização falha se o frontend não estiver disponível; em desenvolvimento, ainda é possível executar somente a API.

Para desenvolvimento com atualização automática, executar o Vite em outro terminal:

```sh
rtk proxy npm --prefix web run dev
```

Nesse modo, iniciar o Go com `PUBLIC_BASE_URL=http://localhost:5173`. O Vite encaminha `/v1` e `/health` à API em `http://127.0.0.1:8080`, preservando a origem do navegador para a proteção CSRF. `API_PROXY_TARGET` permite mudar o destino local.

### Organização

```text
web/src/domain/          contratos da plataforma e estados de verificação
web/src/infrastructure/ adaptador HTTP, cookies e CSRF em memória
web/src/application/    sessão e coordenação de ações
web/src/components/     controles acessíveis e componentes compartilhados
web/src/pages/          telas públicas, conta e administração
web/e2e/                testes em navegador e fixture Go exclusiva de teste
web/scripts/            inicialização isolada da stack E2E
```

As páginas consomem `PlatformGateway`; apenas `main.tsx` escolhe o adaptador HTTP concreto. Credenciais não são salvas em `localStorage` ou `sessionStorage`. O rascunho local é uma ação explícita, com aviso de que ainda não é enviado. As fontes são distribuídas localmente com suas licenças em `web/public/licenses`.

## Desenvolvimento local com Docker

Requer Docker Desktop/Engine funcionando. O Compose usa credenciais conhecidas **somente para desenvolvimento** e publica as portas apenas em localhost. Nenhum provedor de nuvem é necessário.

```sh
rtk proxy docker compose up -d db
rtk proxy docker compose run --rm --build migrate
rtk proxy docker compose up --build api
```

Frontend e API em `http://localhost:8080`. O Compose usa `@naosemetanobrasil` como destinatário oficial dos códigos de vínculo. `INSTAGRAM_OFFICIAL_HANDLE` permite substituir esse destino em outro ambiente. Testes E2E continuam usando um perfil fictício e não enviam mensagens ao Instagram. A prévia executada com `--preview` usa o perfil oficial; o banco dessa prévia continua descartável.

Criar administrador em outro terminal; o comando solicita senha e valida o cadastro do autenticador antes de persistir a conta:

```sh
rtk proxy docker compose run --rm admin -username administrador
```

O comando mostra o endereço `otpauth://` e os códigos de recuperação. Guardar de forma privada. Por padrão, cria uma conta nova.

Para tornar uma conta existente administradora, execute em um terminal privado, com as mesmas variáveis de banco e chave TOTP da API:

```sh
rtk proxy docker compose run --rm admin -promote -username seu_usuario
# Sem Docker:
rtk proxy go run ./cmd/admin -promote -username seu_usuario
```

Essa operação exige a senha atual e a configuração de um novo autenticador TOTP. Mantém ID, senha, vínculos e códigos de recuperação, registra auditoria e encerra todas as sessões anteriores. Entre novamente pela opção **Acesso administrativo** usando o próximo código do autenticador. Não permite substituir o autenticador de uma conta que já seja administradora. Não existe endpoint público para conceder esse papel; o operador precisa ter acesso confiável à configuração do backend. Não envie senha, chave do autenticador ou códigos de recuperação pelo chat.

Parar os containers preservando os dados locais:

```sh
rtk proxy docker compose down
```

## Executar sem containers

Requer Go 1.27.2+ e PostgreSQL. O `go.mod` exige pelo menos essa versão corrigida, também fixada no Dockerfile. A API lê variáveis do processo e **não carrega `.env` automaticamente**. Configurar as variáveis listadas em `.env.example`; para localhost usar `APP_ENV=development`, origem HTTP local e a conexão local do banco.

```sh
rtk go run ./cmd/migrate
rtk go run ./cmd/api
rtk go run ./cmd/admin -username administrador
```

Migrações são executadas explicitamente com credencial própria. A API não deve receber permissão para criar tabelas. Em produção, aplicar [deploy/supabase-runtime.sql](deploy/supabase-runtime.sql) após migrar: o script concede as operações necessárias ao usuário dedicado `nsm_api`, preservando o histórico de migrações privado. Definir a senha e habilitar o login separadamente, conforme [DEPLOY_RENDER.md](DEPLOY_RENDER.md), sem expor o schema na Data API.

No Render, se `PUBLIC_BASE_URL` não estiver definida, o backend usa a origem HTTPS de `RENDER_EXTERNAL_URL`. Após validar DNS e certificado do domínio próprio, definir explicitamente `PUBLIC_BASE_URL=https://naosemetanobrasil.online`.

## Fluxo HTTP do vínculo

Contrato em [openapi/openapi.json](openapi/openapi.json).

1. `POST /v1/auth/register` com `username`, `password` e `display_name` opcional. Guardar os códigos de recuperação retornados.
2. `POST /v1/auth/login` com usuário e senha. Administradores informam também `totp`. Guardar o cookie; o corpo contém `csrf_token`.
3. `POST /v1/me/instagram/challenges` com `{"handle":"@pessoa"}`. Enviar o cookie e o header `X-CSRF-Token`. A resposta contém código de uso único, destinatário oficial e link para abrir o direct.
4. Enviar o código pelo Instagram ao perfil oficial. Informar o envio em `POST /v1/me/instagram/challenges/{id}/sent`; isso **não verifica o perfil**.
5. No modo `webhook`, a Meta entrega a mensagem assinada a `POST /v1/webhooks/instagram`; a API consulta o remetente e registra a conferência automaticamente. No modo `manual`, um administrador confere a caixa oficial e chama `POST /v1/admin/verifications/observe` com `code`, `observed_handle` e referência curta. A API não aceita screenshot como prova automática.
6. A pessoa consulta `GET /v1/me/instagram/challenges/{id}`, confere o perfil observado e chama `POST /v1/me/instagram/challenges/{id}/confirm`.
7. `GET /v1/me` retorna o perfil vinculado e o token CSRF da sessão. A ligação permanece privada nesta etapa.

Sem cookie válido, operações privadas retornam `401`; sem CSRF em mutações, `403`. Outro usuário não pode consultar ou confirmar um desafio alheio. No modo manual, a ligação se refere ao `@` conferido naquela data; não há ID numérico estável obtido da Meta. Administrador não pode conferir o próprio vínculo.

## Testes

Validação da conferência automática em 16/10/2026: testes Go com PostgreSQL real e `-race`, `go vet`, 9 testes de frontend e 5 E2E passaram. O cenário automático usa webhook assinado e consulta de identidade sintética, sem tráfego para a Meta. A entrega real depende da ativação descrita em [INSTAGRAM_INTEGRATION.md](INSTAGRAM_INTEGRATION.md).

Testes unitários e de componentes do frontend:

```sh
rtk proxy npm --prefix web test
```

E2E completo, com navegador, API Go executada como processo separado e PostgreSQL real local:

```sh
rtk proxy npm --prefix web exec -- playwright install chromium
rtk proxy env TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:55432/naosemeta_test?sslmode=disable' npm --prefix web run test:e2e:stack
```

O banco `naosemeta_test` deve existir. O runner recusa bancos externos, aplica migrações, cria uma conta administrativa sintética com TOTP, faz o build do frontend e da API, inicia a API em uma porta livre, executa os testes e encerra o processo. Não apaga registros existentes. Use um banco descartável. A mensagem de Instagram é simulada pela conferência administrativa local; nenhuma mensagem é enviada à Meta.

O E2E cobre cadastro e proteção dos códigos, retomada de desafio sem reexibir o código, vínculo e conferência, recuperação e login, mural de exemplos, rascunho local e uso em celular. `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH` permite escolher um Chromium já instalado para os testes. Não é necessário usar serviços de nuvem.

Validação do frontend em 16/10/2026: 9 testes unitários/de componentes e 4 E2E passaram, incluindo as larguras de 360 e 390 pixels em Chromium. Build de produção, checagem TypeScript e `npm audit` passaram; a auditoria npm retornou zero vulnerabilidades conhecidas. Os testes Go com PostgreSQL real e `-race` também passaram após integrar o frontend. O build da imagem Docker continua sem validação neste ambiente; sua configuração Compose foi conferida.

Testes e análise do Go:

```sh
rtk go test -race ./...
rtk go vet ./...
rtk go build ./cmd/...
```

Testes de integração exigem `TEST_DATABASE_URL` apontando para um banco **descartável**, em localhost, cujo nome termine em `_test`. A suíte recusa endereços externos e não apaga bancos existentes. Ela aplica migrações, cria registros isolados e testa recuperação concorrente, vínculo concorrente, TOTP, quotas e o fluxo completo HTTP.

Exemplo, após criar um banco local `naosemeta_test`:

```sh
rtk proxy env TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:55432/naosemeta_test?sslmode=disable' go test -race -tags integration ./...
```

Sem `TEST_DATABASE_URL`, a execução com `-tags integration` falha explicitamente. Os testes comuns não precisam de banco.

Validação desta entrega em 16/10/2026: suíte completa com PostgreSQL 17.11 local descartável e detector de corrida, `go vet` e compilação dos três comandos passaram usando Go 1.27.2. O Compose foi validado sintaticamente; o build da imagem não foi executado, pois o Docker local não respondeu. A conferência real da caixa do Instagram continua sendo uma operação humana.

O `govulncheck` concluiu sem vulnerabilidades em pacotes importados ou chamadas do projeto. Há um aviso no módulo `golang.org/x/crypto` relativo ao pacote OpenPGP descontinuado, que não é importado pela aplicação; usamos apenas Argon2id. A revisão levou à atualização do Go e das dependências antes da validação final.

## Hospedagem gratuita e segredos

Revisão de segurança, correções e limitações de validação em [SECURITY_REVIEW.md](SECURITY_REVIEW.md).

O desenho continua sendo Render Free + Supabase Free, conforme [BACKEND.md](BACKEND.md). Nenhum recurso pago é necessário para rodar localmente.

A conta do Render foi criada pelo responsável. A configuração para publicação está em [render.yaml](render.yaml), com o passo a passo em [DEPLOY_RENDER.md](DEPLOY_RENDER.md). A conferência automática por webhook está implementada; a ativação e as dependências estão em [INSTAGRAM_INTEGRATION.md](INSTAGRAM_INTEGRATION.md). A entrega real pela Meta e a publicação ainda estão pendentes.

- Produção exige HTTPS, TLS PostgreSQL com `sslmode=verify-full`, pepper de pelo menos 32 bytes e chave TOTP independente de 32 bytes em base64.
- Usar um gerenciador de segredos ou variáveis do provedor; não versionar valores reais.
- Confiar em `X-Forwarded-For` apenas se `TRUSTED_PROXY_CIDRS` contiver os endereços reais dos proxies autorizados. O padrão ignora esse header. Não configurar redes universais.
- Antes de hospedar atrás de um proxy, confirmar seus endereços e testar os limites por IP; sem essa configuração, pessoas atrás do mesmo proxy podem compartilhar a cota.
- A API não depende de arquivos locais persistentes. O banco deve estar migrado antes de servir usuários.
- O histórico agora fica em `app.nsm_migrations`, com RLS e acesso restrito ao migrador. O runner move automaticamente o histórico legado de `public`, sem alterar checksums. Aplicar também `003_private_schema.sql` antes da publicação.
- Produção usa o cookie `__Host-nsm_session`; desenvolvimento HTTP usa `nsm_session`. Sessões de uma versão antiga em produção exigirão novo login.
- Desenvolvimento nativo escuta em `127.0.0.1` por padrão. Não definir `LISTEN_HOST=0.0.0.0` fora de um container deliberadamente isolado sem avaliar a exposição.
- Planos gratuitos podem suspender o serviço e restringir cotas. Não cadastrar pagamento no Render nem habilitar upgrades/add-ons no Supabase.
- Nenhuma automação de CI paga ou criação de infraestrutura é habilitada pelo projeto.

Gerar dois segredos independentes e guardar fora do repositório:

```sh
rtk proxy openssl rand -hex 32
rtk proxy openssl rand -base64 32
```

O primeiro pode ser usado como pepper, e o segundo como chave TOTP. A API rejeita as chaves conhecidas do Compose em produção. Guardar ambos com o backup: trocar o pepper invalida sessões, desafios e códigos de recuperação existentes; trocar a chave TOTP sem recifrar os registros impede o login dos administradores.

## Próximas etapas

Persistência de relatos, mural real, participações, imagens e revisão editorial ainda não foram implementadas. As telas públicas atuais são prévias identificadas. A interface de conferência administrativa e o recebimento automático por webhook já funcionam. A integração real com a Meta depende das credenciais, permissões e callback HTTPS.

A limpeza atual é limitada a sessões expiradas e buckets de rate limit, acionada por tráfego real. Antes de abrir um piloto público, completar retenção de desafios/auditoria, exclusão de conta, backups/restauração, controle global de cotas e os demais critérios de aceite em `BACKEND.md`. Os requisitos do documento completo são o plano do produto, não uma declaração de que todo o MVP está pronto.
