# Revisão de segurança — 16/10/2026

Revisão do código Go/React, migrações, webhook, configuração de publicação, bundle estático e prévias locais. Os problemas reproduzidos abaixo foram corrigidos e receberam testes de regressão. Não foram encontradas credenciais reais nos arquivos examinados; isso não constitui garantia de ausência de vazamentos em ambientes que não foram inspecionados.

## Problemas corrigidos

| ID | Prioridade | Constatação e condição de exploração | Correção |
| --- | --- | --- | --- |
| S01 | P2 | As duas prévias escutavam em `*:63584` e `*:65318`. O endereço mostrado era localhost, mas o listener podia aceitar conexões da rede, conforme firewall e roteamento. Como desenvolvimento usa HTTP e cookies sem `Secure`, isso ampliava indevidamente a exposição dos dados de teste. | Desenvolvimento agora usa `127.0.0.1` por padrão e o runner fixa esse endereço. As duas instâncias abertas foram reiniciadas nas mesmas portas, com snapshot/restauração privados, chaves e dados preservados. `lsof` confirmou listeners IPv4 em loopback. O Compose explicita o bind interno do container e publica suas portas somente no loopback do host. |
| S02 | P2 | A validação de `DATABASE_URL` verificava somente o primeiro `sslmode`. URLs com `sslmode=verify-full&sslmode=disable` ou `&ssl=true` passavam nessa validação; o parser pgx aplica parâmetros posteriores e aliases. Isso dependia de uma configuração de conexão incorreta ou controlada por terceiro, não de um endpoint para alterar a URL. | Produção exige um único `sslmode=verify-full`, rejeita parâmetros repetidos e aceita uma lista limitada de opções. Overrides de host, credenciais, service file e aliases TLS são bloqueados. O teste reproduziu a aceitação anterior e passou depois da correção. |
| S03 | P2, condicional | O histórico de migrações era criado em `public.nsm_migrations`, sem proteção explícita. Em um Supabase com grants padrão amplos e `public` exposto pela Data API, ele poderia ficar acessível ou ser alterado. Nenhum acesso externo real foi constatado: o ambiente Supabase não foi inspecionado. | O runner move o histórico existente para `app.nsm_migrations`, preserva os checksums, habilita RLS sem políticas para usuários comuns e revoga acesso de PUBLIC e dos papéis de API conhecidos. A migração 003 também revoga acesso de `anon`/`authenticated` ao schema privado. Um teste com papel sem privilégios confirmou que um grant de SELECT não expõe o histórico por causa da RLS. |
| S04 | P2 | O rascunho local usava uma única chave por aba e permanecia após logout, permitindo que outra conta na mesma aba visse o texto anterior. Uma resposta de restauração iniciada durante o logout também podia restaurar estado privado na interface após a saída, embora a sessão já estivesse revogada no servidor. | Rascunhos agora são separados entre visitante e ID do usuário. O rascunho de visitante acompanha o login, enquanto logout/expiração limpam o estado autenticado. A saída invalida também respostas concorrentes. Testes de frontend e E2E cobrem conservação do rascunho no login, limpeza no logout e troca de conta. |

## Reforços aplicados

- **Cookies de produção:** usam `__Host-nsm_session`, `Secure`, `HttpOnly`, `SameSite=Lax`, `Path=/` e nenhum `Domain`. O nome antigo não é aceito como fallback em produção. Desenvolvimento HTTP continua usando `nsm_session`. Isso reduz a possibilidade de injeção de cookie por subdomínio; não significa que uma exploração desse tipo tenha sido constatada.
- **Chaves independentes:** produção rejeita pepper e chave TOTP iguais, inclusive codificados em hex/base64. O verify token do webhook também não pode reutilizar o App Secret ou sua codificação. O token sintético de E2E é proibido em produção.
- **Arquivos estáticos:** o processo usa `os.Root.FS()` para impedir que um symlink de asset saia do diretório do frontend. A exploração exigiria introduzir esse arquivo no servidor; não existe upload público de assets nesta versão.
- **Cabeçalhos:** respostas da API, inclusive erros, recebem CSP restritiva, proteção contra enquadramento e `Referrer-Policy: no-referrer`. Produção envia HSTS sem incluir subdomínios nem solicitar preload. O frontend mantém sua CSP própria para os recursos locais.
- **Artefatos de teste:** gravação automática de traces e screenshots de falha foi desativada, pois pode registrar senhas, cookies ou códigos nos fluxos de autenticação. Capturas explícitas dos testes ficam limitadas às telas escolhidas sem códigos secretos. Os testes usam credenciais sintéticas e banco local.

## Tokens, dados privados e controles conferidos

- Varredura por padrões de chaves privadas, JWTs, tokens de provedores e URLs PostgreSQL no código e arquivos gerados, excluindo dependências binárias. As URLs encontradas eram exemplos ou credenciais locais conhecidas de desenvolvimento. O workspace não contém `.env` real; há apenas `.env.example`.
- Build com canários sintéticos em variáveis privadas do servidor: nenhum canário apareceu nos arquivos públicos examinados. Não há sourcemap no build de produção.
- A varredura final examinou 102 arquivos de texto, sem padrões de tokens reais de provedores/chaves privadas. Não restaram traces ZIP de testes nem backups temporários das reinicializações. Os endpoints locais retornaram 404 para `.env`/`.git` e 401 para dados de conta e fila administrativa sem credenciais.
- Tokens de sessão, códigos de recuperação e códigos de vínculo são gerados criptograficamente e gravados no banco como HMAC. O token de sessão não aparece no JSON de login. O segredo TOTP é cifrado com AES-GCM; seu cadastro e os códigos de recuperação são exibidos intencionalmente no terminal privado do operador.
- O navegador mantém CSRF em memória. Códigos de vínculo/recuperação não são persistidos automaticamente em localStorage ou sessionStorage. Copiar ou baixar códigos é uma ação explícita da pessoa. Rascunhos permanecem em sessionStorage, sem criptografia: isolamento lógico por conta e limpeza não protegem contra alguém com acesso ao navegador/DevTools.
- Consultas SQL usam parâmetros. Consultar e confirmar desafios exige o usuário proprietário; endpoints administrativos exigem papel admin. Recuperação consome o código e revoga sessões em transação. TOTP rejeita reutilização de contador.
- O webhook valida HMAC sobre os bytes originais, conta destinatária e ID do remetente. A consulta autenticada de perfil usa origem fixa, não segue redirects e não inclui token na URL. Código, mensagem em claro, ID remoto privado e detalhes de erros de upstream não entram nas respostas públicas ou logs da API.
- Conferência, recibo e auditoria do webhook são transacionais. Entregas repetidas e concorrentes, expiração, cancelamento, remetente incorreto e vínculo duplicado por ID remoto estão cobertos por testes.
- Limites de corpo, tempo de requisição, chamadas externas, concorrência Argon2 e tentativas de autenticação estão implementados. Headers de proxy somente são confiados quando o endereço do proxy está explicitamente configurado.
- `go list` confirmou que o binário normal usa `instagram_profiles.go` e não inclui o adaptador sintético `instagram_profiles_e2e.go`. Docker compila sem a tag E2E.

## Verificações executadas

| Verificação | Resultado |
| --- | --- |
| Testes Go com PostgreSQL local e `-race` | Passaram, incluindo migração do histórico e isolamento de permissões |
| Regressões de configuração TLS/chaves | Passaram; os casos TLS falhavam antes da correção |
| `go vet ./...` | Passou |
| Testes de frontend | 12 passaram |
| E2E com API Go e PostgreSQL reais | 6 passaram; identidade Meta simulada, sem enviar mensagens reais |
| Build TypeScript/Vite e canários privados | Passaram; nenhum canário ou sourcemap público |
| `npm audit` | 0 vulnerabilidades conhecidas nas dependências examinadas |
| `govulncheck` | 0 falhas alcançáveis e 0 avisos em pacotes importados |
| Aviso de módulo Go | GO-2026-5932, sobre `golang.org/x/crypto/openpgp`, que não é importado; o projeto usa Argon2id |
| YAML Render/Compose | Sintaxe e plano Free conferidos; publicação de portas Docker somente em loopback |
| Instâncias locais | Ambas em `127.0.0.1`, mantendo portas e dados existentes |

## Limites da revisão e pendências de publicação

Não foi inspecionado histórico Git: este workspace não contém `.git`. Não foram inspecionados painéis Render/Supabase/Meta, segredos cadastrados nesses provedores, backups remotos, logs de infraestrutura, DNS/TLS do domínio publicado ou permissões efetivas da conta Meta. A imagem Docker não foi construída aqui; o daemon local não estava disponível na preparação anterior. Os resultados não substituem um teste do ambiente publicado.

Antes de abrir um piloto público:

1. Aplicar as migrações com o papel de migração e usar um papel diferente, sem privilégios de proprietário/superusuário/bypass RLS, na API. Conceder ao runtime apenas as permissões necessárias. Não conceder acesso ao histórico de migrações, nem expor `app` na Data API. Verificar a configuração efetiva no Supabase; a RLS do histórico não restringe proprietários ou superusuários.
2. Verificar no ambiente real que cookies, HTTPS, origens, proxies, grants do banco e escopos Meta correspondem ao código. O Blueprint continua em modo manual até cadastrar as credenciais e habilitar o webhook explicitamente.
3. Completar as funções já pendentes do produto: exclusão/retensão de contas, desafios, recibos e auditoria; backup/restauração; limites globais para as cotas gratuitas. Não há monitoramento contínuo de vulnerabilidades configurado por esta revisão.
4. Repetir a revisão ao implementar uploads, relatos públicos e participações. Essas funcionalidades ainda não existem, e terão novas superfícies de exposição e abuso.

Nenhuma rotação de credencial real foi solicitada: a varredura não encontrou um segredo real exposto. Se um segredo real tiver sido compartilhado fora deste workspace, ele precisa ser revogado no provedor; apagar o texto de um arquivo não revoga a credencial.

Referências: [OWASP — sessões e cookies](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html), [aviso Go GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932).
