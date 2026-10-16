# Não se Meta no Brasil — especificação do backend

> Atualização de implementação: o vínculo agora também suporta conferência automática por webhook assinado, sem segundo código ou envio automático de DM. O modo manual permanece disponível. Fluxo, ativação e limites estão em [INSTAGRAM_INTEGRATION.md](INSTAGRAM_INTEGRATION.md); as seções de conferência manual abaixo descrevem a alternativa administrativa.

Data: 16/10/2026. Status: autenticação, vínculo manual e frontend inicial implementados. Relatos e mural público real permanecem planejados. Consulte `README.md` para o estado real, execução e limitações.

## 1. Objetivo

Criar uma plataforma pública para registrar acontecimentos em redes sociais, anexar evidências e receber contribuições de outras pessoas. Cada participante terá uma conta própria na plataforma e poderá comprovar o controle de seu perfil do Instagram por mensagem direta ao perfil oficial do projeto.

O backend será em Go. A infraestrutura deverá permanecer em planos gratuitos, sem contratação automática de recursos pagos. Ao atingir limites, a aplicação deverá reduzir funcionalidades ou ficar temporariamente indisponível.

O vínculo comprova o controle do perfil no momento da verificação. Não comprova identidade civil nem a veracidade dos relatos. O perfil de quem relata pode ser diferente do perfil envolvido no acontecimento.

## 2. Decisões de infraestrutura e custo

| Componente | Escolha | Custo previsto no MVP |
| --- | --- | --- |
| API e painel administrativo básico | Go em um único Render Web Service Free | Zero dentro das cotas |
| Banco de dados | PostgreSQL no Supabase Free | Zero dentro das cotas |
| Imagens e miniaturas | Supabase Storage Free | Zero dentro das cotas |
| Login e sessões | Implementados na API Go | Sem fornecedor adicional |
| Verificação do Instagram | Direct recebido e conferido por administrador | Sem serviço pago; exige trabalho manual |
| Entrada de webhooks, em fase posterior | Supabase Edge Function | Dentro da cota gratuita; opcional |
| Endereço inicial e HTTPS | Subdomínio disponibilizado pelo Render | Sem compra de domínio |

As condições abaixo foram consultadas em 16/10/2026. Revalidar antes da implantação: planos gratuitos podem mudar.

### Render Free

- Disponibiliza 750 horas de instância por workspace/mês, compartilhadas entre serviços gratuitos.
- Suspende um serviço ocioso após 15 minutos; a retomada pode levar aproximadamente um minuto.
- O disco é temporário: reinícios e suspensões podem apagar arquivos locais.
- O PostgreSQL gratuito do próprio Render expira após 30 dias; não será utilizado.
- O provedor apresenta o plano como destinado a experimentos e projetos pessoais. Este MVP deve ser tratado como piloto com disponibilidade limitada.

Fonte: [Deploy for Free](https://render.com/docs/free).

**Controle financeiro:** usar uma conta/workspace sem forma de pagamento cadastrada. O Render informa que excedentes podem ser cobrados quando existe uma forma de pagamento; sem ela, os serviços ou builds são suspensos conforme o recurso excedido. Se o cadastro exigir um meio de pagamento sem garantia de bloqueio de cobrança, interromper a implantação e revisar a escolha.

Fonte: [Render FAQ — cobrança em serviços gratuitos](https://render.com/docs/faq).

### Supabase Free

- Banco: 500 MB por projeto.
- Arquivos: 1 GB.
- Transferência: 5 GB de egress e 5 GB de cached egress, com contabilização própria de cada categoria.
- Projetos gratuitos podem ser pausados após uma semana de baixa atividade.
- Backups automáticos não estão incluídos no plano gratuito.
- Edge Functions, se utilizadas: 500 mil invocações incluídas.

Fontes: [Preços](https://supabase.com/pricing), [Cobrança e cotas](https://supabase.com/docs/guides/platform/billing-on-supabase), [Pausa de projetos](https://supabase.com/docs/guides/platform/free-project-pausing).

Usar organização exclusivamente Free, sem upgrade, add-ons ou recursos pagos. Exceder cotas pode causar restrições, inclusive indisponibilidade; o desenho não depende de upgrade para continuar. Fonte: [Billing FAQ](https://supabase.com/docs/guides/platform/billing-faq).

### Regras obrigatórias para manter custo zero

1. Não cadastrar forma de pagamento no Render, não contratar planos pagos e não habilitar cobrança por uso.
2. Não usar créditos promocionais ou trials com vencimento como infraestrutura permanente.
3. Não contratar domínio, SMTP, SMS, Redis, workers dedicados, IA ou ferramentas de observabilidade pagas.
4. Usar URLs gratuitas dos provedores. Um domínio já existente poderá ser configurado depois, mas sua renovação não faz parte do orçamento zero.
5. Não criar pings artificiais para impedir a suspensão dos serviços.
6. Limites esgotados resultam em restrição, manutenção ou indisponibilidade, nunca em compra automática.
7. Conferir cotas nos painéis dos provedores durante a operação. Contadores internos não medem todo o tráfego faturável.

**Limite da proposta:** custo recorrente de infraestrutura zero dentro das condições gratuitas. Não há garantia de disponibilidade contínua, escala ilimitada ou permanência dos planos. Trabalho humano, conexão à internet e equipamentos existentes não estão incluídos nessa estimativa.

## 3. Arquitetura

```text
Navegador
    |
    v
API Go + painel administrativo — Render Free
    |                            |
    v                            v
PostgreSQL — Supabase Free    Storage privado — Supabase Free

Instagram da pessoa -> Direct -> Instagram oficial do projeto
                                      |
                             Conferência administrativa
                                      |
                             Registro do vínculo na API
```

- Monólito modular, API REST em JSON, prefixo `/v1`.
- Arquitetura hexagonal e princípios SOLID: domínio puro, casos de uso, portas pequenas e adaptadores substituíveis. Dependências apontam para o núcleo; a composição concreta fica em `cmd/`.
- Go em versão estável e suportada, registrada no `go.mod` na implementação.
- HTTP com `net/http`; PostgreSQL com `pgx/v5` e SQL explícito.
- Migrações SQL versionadas; ferramenta gratuita como `goose`.
- Logs estruturados com `log/slog` em stdout.
- Sem microserviços, fila externa, Elasticsearch ou Redis no MVP.
- Processos e arquivos em memória/disco local são descartáveis; dados persistentes ficam no Supabase.
- Painel administrativo básico servido pela própria aplicação. O frontend público poderá inicialmente compartilhar a mesma origem, simplificando cookies e CSRF.

### Conexão com o banco

Usar a conexão Supavisor em modo session, compatível com IPv4, obtida no painel do Supabase. Habilitar TLS com validação de certificado; pool inicial de até cinco conexões, com timeout e reconexão. Usar usuário de aplicação com privilégios mínimos e credencial separada para migrações. Não expor credenciais do banco nem chave privilegiada de Storage ao navegador.

As tabelas internas devem ficar em schema não exposto pela Data API. O navegador acessa as regras de negócio pela API Go. Não presumir que RLS protege consultas feitas com credenciais que a ignoram.

Fonte: [Conexões PostgreSQL no Supabase](https://supabase.com/docs/guides/database/connecting-to-postgres).

## 4. Escopo funcional do MVP

- Cadastro, login, logout e recuperação por códigos de emergência.
- Vínculo de perfil pessoal ou profissional do Instagram por direct.
- Rascunho e envio de relato com formulário curto.
- Mural público, filtros e página de cada relato publicado.
- Anexos de imagem e links externos opcionais.
- Participações: acompanhar, testemunhar o mesmo caso, registrar experiência semelhante ou acrescentar evidência.
- Fila administrativa de verificação de perfis e revisão de relatos.
- Histórico de mudanças relevantes, correções e respostas da plataforma envolvida.
- Denúncia de abuso ou exposição de dados em conteúdo publicado.

Ficam para depois: login oficial de outras redes, automação dos directs, OCR, IA, vídeo hospedado, comentários livres, notificações externas e aplicativos nativos. Essas funcionalidades não podem introduzir custo obrigatório.

## 5. Cadastro e acesso independentes da Meta

O Instagram é um perfil vinculado à conta local, não o único meio de entrar na plataforma. A suspensão da rede social não deve impedir o acesso aos relatos existentes.

### Cadastro

- Solicitar nome de usuário local e senha; nome público opcional.
- Não exigir CPF, telefone ou e-mail no MVP.
- Armazenar senha com Argon2id e salt aleatório; parâmetros calibrados para a instância gratuita e concorrência limitada.
- Gerar códigos de recuperação aleatórios e de uso único, exibidos apenas na criação ou regeneração. Armazenar somente hashes.
- Recuperação com código válido redefine a senha, invalida sessões anteriores e consome o código atomicamente.
- Sem senha nem código de recuperação, não oferecer redefinição automática baseada apenas em nome, print ou `@`.

### Sessões

- Token opaco, aleatório, armazenado no banco somente como hash.
- Cookie `HttpOnly`, `Secure`, `SameSite=Lax`, prazo inicial de sete dias; rotacionar após login e mudanças de credenciais.
- Validar Origin e token CSRF em operações mutáveis autenticadas por cookie.
- Restringir CORS às origens configuradas; não combinar credenciais com origem `*`.
- Respostas de login devem evitar revelar se o usuário existe.
- Administração exige papel específico e segundo fator TOTP, sem dependência de SMS.
- Primeiro administrador criado por comando local autenticado com credencial de operação, sem endpoint público de promoção.

## 6. Vínculo com o Instagram por direct

### Experiência da pessoa

1. Autenticada na plataforma, informa o `@` que quer vincular.
2. Recebe um código aleatório gerado pela nossa plataforma e o nome exato do perfil oficial destinatário.
3. Copia o código e abre a conversa com o perfil oficial do projeto.
4. Envia o código a partir do Instagram informado.
5. Acompanha o andamento na própria plataforma.
6. Após conferência, confirma o perfil remetente identificado e conclui o vínculo.

A primeira versão é manual: mostrar “Aguardando conferência”, sem prometer confirmação imediata. A pessoa não precisa alterar a bio nem transformar sua conta em profissional.

O código é nosso, não é uma senha nem um código de autenticação emitido pelo Instagram. Seu envio autoriza somente o vínculo informado, sem autorizar publicação ou ações na rede social.

### Desafio de verificação

- Código com pelo menos 128 bits aleatórios, codificado em formato fácil de copiar, com prefixo `NSM-`.
- Vinculado a `user_id`, rede, perfil esperado e finalidade de vinculação.
- Validade de 24 horas para permitir conferência manual; configurável. O TTL começa na emissão.
- Armazenar hash/HMAC do código; retornar o texto uma vez e não registrá-lo em logs.
- Novo desafio invalida o anterior ainda pendente para o mesmo usuário e rede.
- Até três emissões por hora e dez por dia por usuário, além do limite por IP.
- A pessoa informa que enviou o direct; essa ação não confirma o vínculo nem renova a validade.
- Estados: `pending`, `awaiting_confirmation`, `verified`, `expired`, `rejected`, `cancelled`.
- Revalidar prazo tanto na conferência quanto na confirmação final. Se expirar, emitir novo desafio.

### Conferência manual

1. Administrador acessa a mensagem recebida na caixa do perfil oficial do projeto.
2. Identifica o remetente pelo perfil real associado à conversa, não por texto escrito na mensagem ou por screenshot fornecido pelo usuário.
3. Informa o código no painel protegido; o backend encontra o desafio por seu hash.
4. Confere correspondência com o `@` esperado e registra remetente, método, horário, administrador e referência mínima da conversa.
5. Marca `awaiting_confirmation`; o usuário vê o perfil identificado e confirma o vínculo em sua sessão local.

Código desconhecido, expirado, já consumido, remetente divergente ou perfil já vinculado a outro usuário devem impedir a conclusão. Um conflito não pode transferir automaticamente o perfil.

No modo manual, o identificador numérico estável do Instagram pode não estar disponível. Registrar essa limitação: a verificação é referente ao `@` observado na data. Não inventar IDs nem inferir continuidade quando um username muda de dono. Alteração de `@` exige nova verificação. Registrar um ID oficial com seu contexto de escopo quando ele se tornar disponível.

### Regras de persistência e exibição

- Normalizar `@` removendo prefixo e espaços e convertendo para minúsculas, preservando o valor de exibição.
- Um perfil ativo por usuário e rede no MVP; um perfil normalizado não pode ficar ativo em duas contas locais.
- Quando houver ID oficial, aplicar também unicidade por rede, contexto de escopo e ID externo.
- Verificar desafio, reservar vínculo, consumir código e registrar auditoria em transação, com locks e constraints contra concorrência.
- Guardar método (`manual_dm` ou, futuramente, `api_dm`), data e estado do vínculo.
- Exibir “Controle do perfil confirmado”, com data e método. Não usar “Identidade verificada” ou “Relato comprovado”.
- A pessoa escolhe entre nome local público ou `@` público; a verificação interna pode continuar privada.
- Desvincular revoga o vínculo atual, sem apagar silenciosamente a autoria ou reatribuir relatos antigos.
- Um novo vínculo não muda retroativamente o perfil associado aos relatos anteriores; manter histórico interno mínimo, com política de exclusão de dados.

### Contas inacessíveis

Permitir salvar rascunhos e enviar documentação para revisão excepcional quando a conta já estiver suspensa. Não marcar como vínculo confirmado pelo método de direct. A publicação excepcional depende de decisão administrativa registrada e mantém a indicação de vínculo não confirmado.

## 7. Automação dos directs — fase posterior

A integração de mensagens permite que uma conta profissional receba mensagens de usuários do Instagram. A conta profissional será a do projeto. Permissões e acesso avançado para participantes externos precisam estar disponíveis antes da ativação; validar também a adequação do uso às políticas vigentes. Não prometer aprovação da Meta.

Fontes: [Instagram Messaging na coleção oficial da Meta](https://www.postman.com/meta/instagram/documentation/6yqw8pt/instagram-api?entity=request-23987686-3a63181c-e5c6-4890-96dd-e4d425bef5e0), [Requisitos de acesso a conversas](https://www.postman.com/meta/messenger-platform-api/folder/22794852-255610cd-47f5-4f4d-b3fa-71aec360be9a).

### Recepção compatível com a API suspensa

Não apontar a entrada principal de webhook diretamente para o Go no Render Free: a retomada após ociosidade pode ultrapassar o prazo esperado pelo emissor.

Proposta opcional: uma Supabase Edge Function recebe o webhook, valida autenticidade e grava os eventos relevantes em `instagram_inbox_events`. Só confirma recebimento depois da persistência. A API Go processa os eventos quando estiver ativa, ao consultar o status de verificação ou por ação administrativa. Não há worker pago nem requisito de processamento instantâneo.

A Edge Function será um adaptador pequeno em TypeScript/Deno, exceção explícita ao backend Go. Toda regra de vínculo e negócio permanece em Go. Se for exigido que também o adaptador seja Go, manter o método manual até encontrar uma hospedagem gratuita adequada à recepção contínua.

Fonte: [Supabase Edge Functions](https://supabase.com/docs/guides/functions).

### Requisitos da automação

- Tratar o handshake `hub.challenge` com segredo próprio de verificação.
- Validar `X-Hub-Signature-256` sobre o corpo original usando o segredo do app e comparação em tempo constante.
- Não confundir JWT da plataforma com autenticação de webhook; o endpoint externo usa validação própria da Meta.
- Validar conta oficial destinatária, tipo de evento e tamanho do payload.
- Persistir somente o mínimo necessário dos eventos de texto que possam corresponder a desafios; não arquivar outras conversas.
- Deduplicar por ID da mensagem e contexto de destino, inclusive em entregas repetidas.
- Resolver o remetente por dados oficiais permitidos; comparar com o perfil esperado. Se os dados necessários não estiverem disponíveis, encaminhar à conferência manual.
- Eventos persistidos antes de `expires_at` podem ser processados depois; a confirmação final exige uma janela própria explícita e curta, por exemplo 24 horas após a validação. Essa regra substitui o prazo único do modo manual apenas quando o evento possui recebimento autenticado e durável.
- Consumir eventos com lock/lease, retries limitados e estado de erro; retomadas não podem criar vínculos duplicados.
- Excluir payloads processados em até sete dias; códigos não devem permanecer em registros históricos.
- Se banco ou cota da função estiverem indisponíveis, não responder sucesso falso; manter fallback manual.

Antes de implementar, conferir a especificação vigente em [Webhooks da Meta](https://developers.facebook.com/docs/graph-api/webhooks/getting-started/). Versão da API, permissões e acesso a campos do remetente devem ser fixados e testados com contas externas reais autorizadas.

## 8. Relatos com preenchimento curto

### Campos de envio

| Campo | Obrigatório? | Regra |
| --- | --- | --- |
| Categoria | Sim | `content_removed`, `account_suspended`, `restriction`, `reach_drop`, `other` |
| Descrição curta | Sim | Entre 10 e 1.000 caracteres |
| Rede envolvida | Sim | Pré-selecionar quando houver contexto; usuário confirma |
| Data aproximada | Não | Não confundir data do envio com data do acontecimento |
| Link do conteúdo/perfil | Não | URL HTTPS, até 2.048 caracteres |
| Até três imagens | Não | Respeitar limites de armazenamento e revisão |

A primeira tela pode começar por imagem ou link. O backend aceita rascunho incompleto, mas exige os campos mínimos ao enviar. Sem OCR ou IA no MVP, não extrair nem inventar informações automaticamente. O frontend pode sugerir a rede pelo domínio de um link e permitir correção.

Relato sem evidência é permitido, mas recebe indicação explícita. Não exigir documento de identificação nem longos questionários.

### Publicação e moderação

- Estados editoriais: `draft`, `pending_review`, `published`, `rejected`, `hidden`, `withdrawn`.
- Envio normal para revisão exige vínculo ativo confirmado; aplicar o fluxo excepcional documentado para conta inacessível.
- No piloto, revisar antes da primeira publicação para evitar exposição involuntária de dados e abuso.
- Estado de apuração é independente: `unreviewed`, `documentation_available`, `under_review`, `response_received`, `closed`.
- “Documentação disponível” indica anexos conferidos quanto à presença, não prova da acusação.
- Editar conteúdo publicado gera revisão pendente; a versão nova não substitui automaticamente a pública.
- Registrar correções, decisões administrativas e respostas com data e responsável.
- Não utilizar filiação, candidato apoiado ou orientação política como condição de cadastro.

### Participações

| Tipo | Significado | Requisito |
| --- | --- | --- |
| `follow` | Quero acompanhar | Conta autenticada |
| `witness` | Testemunhei este mesmo caso | Vínculo confirmado e contexto curto |
| `similar_experience` | Aconteceu algo semelhante comigo | Vínculo confirmado; relato próprio vinculado |
| `evidence` | Tenho evidência adicional | Vínculo confirmado; texto ou anexo, com revisão |

Uma experiência semelhante não aumenta o número de testemunhas daquele caso. Contadores distinguem acompanhamento, testemunhos e contribuições. Não usar popularidade como indicador de veracidade. Agrupamentos de casos são administrativos e reversíveis.

## 9. Arquivos e proteção das cotas

- Aceitar JPEG e PNG; não aceitar vídeo, PDF ou SVG no MVP.
- Limite de entrada: 1 MB por imagem, três imagens por relato ou contribuição, até 4 MB por requisição multipart.
- Validar assinatura do arquivo, dimensões e quantidade de pixels antes de decodificar completamente.
- Gerar miniatura e remover metadados de localização; registrar SHA-256 do arquivo efetivamente armazenado. O hash não prova autenticidade da captura.
- Considerar no orçamento todos os objetos: anexos, miniaturas e eventuais versões públicas revisadas.
- Bucket privado, nomes aleatórios e URLs assinadas de curta duração, emitidas apenas para arquivos autorizados.
- Apenas versões aprovadas para publicação podem ser servidas a leitores públicos. Originais pendentes ficam restritos ao autor e à moderação.
- URLs assinadas já emitidas podem continuar funcionando até expirar; utilizar TTL curto e não prometer revogação instantânea.
- Oferecer prévia e orientação para ocultar dados pessoais antes do envio. A revisão pode rejeitar a imagem; anonimização não deve ser presumida automática.
- Não baixar conteúdo de URLs fornecidas por usuários no MVP; armazenar o link. Isso evita custos de crawling e acesso indevido a endereços internos.
- Reservar bytes em transação antes do upload e reconciliar falhas/objetos órfãos; estado de arquivo: `pending`, `ready`, `rejected`, `deleted`.

### Limites internos iniciais

| Recurso | Alerta interno | Comportamento preventivo |
| --- | --- | --- |
| Objetos armazenados | 700 MB | Bloquear novos uploads a partir de 800 MB, incluindo reservas |
| Banco | 350 MB | A partir de 400 MB, desativar novas escritas públicas e manter operações de recuperação/moderação enquanto houver espaço |
| Transferência | 70% de cada cota | Revisar no painel; reduzir emissão de URLs, anexos e tamanho das listagens |
| Imagens por usuário | 10 MB/dia | Recusar novos uploads acima do limite |
| Relatos por usuário | 5 envios/dia | Preservar rascunhos existentes; bloquear novos envios |

Os limites são decisões do projeto, inferiores às cotas dos provedores; usar unidades explicitamente definidas na implementação. Métricas locais de transferência são estimativas, pois tráfego de Storage pode não passar pelo Go. Manter flags `UPLOADS_ENABLED` e `PUBLIC_WRITES_ENABLED` para bloqueio imediato.

Paginação padrão de 20 itens, máximo de 50, por cursor. Carregar miniaturas sob demanda; respostas públicas não incluem anexos completos em linha. Começar com ordem cronológica e filtros simples, com índices SQL.

## 10. Modelo de dados

Todos os registros usam UUID, timestamps UTC e chaves estrangeiras. Horários de acontecimentos são opcionais e podem ter precisão `day` ou `exact`; exibir conforme fuso do usuário. Definir cascatas de exclusão deliberadamente.

| Tabela | Campos principais |
| --- | --- |
| `users` | `id`, `username_normalized`, `display_name`, `password_hash`, `role`, `status`, `created_at` |
| `admin_totp_credentials` | `user_id`, `encrypted_secret`, `enabled_at` |
| `sessions` | `id`, `user_id`, `token_hash`, `expires_at`, `revoked_at` |
| `recovery_codes` | `id`, `user_id`, `code_hash`, `used_at` |
| `social_profiles` | `id`, `user_id`, `platform`, `handle_normalized`, `external_id`, `external_id_scope`, `verification_method`, `verified_at`, `revoked_at`, `public_visibility` |
| `verification_challenges` | `id`, `user_id`, `platform`, `expected_handle`, `code_hash`, `status`, `expires_at`, `confirmation_expires_at`, `observed_handle`, `reviewed_by`, `reviewed_at`, `consumed_at` |
| `reports` | `id`, `author_id`, `author_social_profile_id`, `platform`, `category`, `description`, `occurred_at`, `date_precision`, `source_url`, `editorial_status`, `investigation_status`, `published_at` |
| `report_revisions` | `id`, `report_id`, `editor_id`, `content`, `status`, `created_at`, `reviewed_by` |
| `participations` | `id`, `report_id`, `user_id`, `social_profile_id`, `type`, `context`, `related_report_id`, `status`, `created_at` |
| `evidence_files` | `id`, `owner_id`, `report_id`, `participation_id`, `object_key`, `thumbnail_key`, `bytes`, `sha256`, `status`, `visibility`, `created_at` |
| `report_updates` | `id`, `report_id`, `actor_id`, `type`, `public_text`, `created_at` |
| `report_groups` / `report_group_members` | Agrupamento administrativo de ocorrências relacionadas |
| `content_flags` | `id`, `report_id`, `participation_id`, `reporter_id`, `reason`, `status`, `created_at` |
| `audit_events` | `id`, `actor_id`, `action`, `entity_type`, `entity_id`, `minimal_metadata`, `created_at` |
| `storage_reservations` | `id`, `user_id`, `bytes_reserved`, `expires_at`, `status` |
| `rate_limit_buckets` | `key_hash`, `action`, `window_start`, `count`, `expires_at` |
| `instagram_inbox_events` | Fase posterior: `id`, `destination_scope`, `message_id`, `sender_id`, `minimal_payload`, `received_at`, `status`, `attempts`, `lease_until` |

### Constraints e índices

- `users.username_normalized` único; hashes de sessão e de desafio únicos.
- Índice parcial único para perfil ativo em `(platform, handle_normalized)` e outro em `(user_id, platform)`.
- Índice parcial único para ID externo ativo quando disponível, incluindo seu escopo.
- Perfil autor de relato/contribuição deve pertencer ao mesmo usuário; validar no banco por chave composta ou no comando transacional.
- Participação única em `(report_id, user_id, type)`; `PUT` idempotente para acompanhar/testemunhar. No MVP, evidências adicionais atualizam a contribuição existente e passam por revisão.
- `similar_experience` exige `related_report_id` acessível; `evidence` exige texto ou arquivo pronto; `witness` exige contexto.
- `evidence_files` deve pertencer a exatamente um relato ou uma participação; anexos temporários usam reserva separada e prazo de limpeza.
- Mural: índice parcial `(published_at DESC, id DESC)` para relatos publicados e índices para rede/categoria.
- Eventos de inbox únicos em `(destination_scope, message_id)`.
- Estados validados por `CHECK` e transições autorizadas por comandos de aplicação.
- Auditoria não contém senhas, códigos, tokens, segredos TOTP ou conversas completas.

## 11. Contrato inicial da API

| Método e rota | Finalidade | Acesso |
| --- | --- | --- |
| `POST /v1/auth/register` | Criar conta e fornecer códigos de recuperação uma vez | Público com limitação |
| `POST /v1/auth/login` | Abrir sessão | Público com limitação |
| `POST /v1/auth/recover` | Redefinir senha usando código de recuperação | Público com limitação |
| `POST /v1/auth/logout` | Revogar sessão | Autenticado |
| `GET /v1/me` | Perfil local, vínculos, pendências e permissões | Autenticado |
| `POST /v1/me/instagram/challenges` | Emitir desafio vinculado ao `@` informado | Autenticado |
| `GET /v1/me/instagram/challenges/{id}` | Consultar estado e perfil observado | Dono |
| `POST /v1/me/instagram/challenges/{id}/sent` | Informar envio, sem validar o perfil | Dono |
| `POST /v1/me/instagram/challenges/{id}/confirm` | Confirmar perfil observado e consumir desafio | Dono |
| `DELETE /v1/me/social-profiles/{id}` | Revogar vínculo | Dono |
| `GET /v1/reports` | Mural publicado com filtros e cursor | Público |
| `GET /v1/reports/{id}` | Relato publicado, histórico e contadores | Público; rascunhos só para dono/moderação |
| `POST /v1/reports` | Criar rascunho | Autenticado |
| `PATCH /v1/reports/{id}` | Alterar rascunho ou propor revisão | Dono |
| `POST /v1/reports/{id}/submit` | Enviar à revisão | Dono com vínculo, ou fluxo excepcional |
| `POST /v1/reports/{id}/withdraw` | Retirar publicação e revogar acesso aos anexos | Dono |
| `POST /v1/reports/{id}/evidence` | Receber imagens dentro da reserva/cota | Dono |
| `GET /v1/evidence/{id}/access` | Emitir URL curta se o arquivo estiver autorizado | Conforme visibilidade |
| `PUT /v1/reports/{id}/participations/{type}` | Criar/atualizar participação | Conforme o tipo |
| `DELETE /v1/reports/{id}/participations/{type}` | Retirar participação própria | Dono |
| `POST /v1/reports/{id}/flags` | Reportar abuso ou exposição de dados | Autenticado |
| `GET /v1/admin/verifications` | Fila de vínculos pendentes | Administrador |
| `POST /v1/admin/verifications/observe` | Registrar conferência da mensagem real | Administrador |
| `POST /v1/admin/verifications/{id}/reject` | Rejeitar conferência com motivo | Administrador |
| `GET /v1/admin/reports` | Fila editorial | Moderador/administrador |
| `POST /v1/admin/reports/{id}/decisions` | Publicar, rejeitar ou ocultar | Moderador/administrador |
| `GET /v1/admin/usage` | Cotas estimadas e flags operacionais | Administrador |
| `GET /health/live` | Processo respondendo | Público, sem detalhes internos |
| `GET /health/ready` | Dependências acessíveis | Público, resposta mínima |

Na fase automática, handshake e recebimento externo ficam na Edge Function; não são endpoints comuns de autenticação por cookie. Processamento de inbox pode ser acionado por comando administrativo autenticado, com lote limitado.

### Convenções

- JSON `snake_case`; UUIDs como strings; timestamps RFC 3339 em UTC.
- Erros: `{ "error": { "code": "...", "message": "...", "request_id": "..." } }`.
- `400`: entrada inválida; `401`: sem sessão; `403`: sem permissão; `404`: não encontrado ou não visível; `409`: conflito; `413`: arquivo grande; `429`: limite; `503`: dependência/cota indisponível.
- Payload JSON máximo inicial de 64 KB, exceto upload com limite próprio.
- `Idempotency-Key` em criação de relatos e envio à revisão; vincular à sessão/usuário, rota e hash do corpo, persistindo resultado por 24 horas.
- Não repetir automaticamente uploads ou mutações sem chave idempotente.
- Documento OpenAPI versionado será um entregável da implementação.

## 12. Abuso, privacidade e operação

- Limitar tentativas de login e recuperação por conta/IP; limitar cadastros, consultas e desafios.
- Persistir contadores relevantes no PostgreSQL para não zerarem a cada retomada; varrer expirados em lotes pequenos quando a API estiver ativa.
- Não aceitar automaticamente cabeçalhos de IP enviados pelo cliente; confiar apenas no proxy configurado.
- Não indexar ou devolver rascunhos, anexos pendentes, códigos de vínculo ou histórico administrativo em respostas públicas.
- Publicar somente conteúdo e perfil explicitamente autorizados; consentimento para perfil público é separado do vínculo interno.
- Oferecer retirada de relato e solicitação de exclusão da conta. Anonimizar autoria pública quando apropriado e definir retenção interna mínima antes do lançamento.
- Manter evidências e correções acessíveis apenas conforme o estado editorial atual, inclusive após cache e URLs expirarem.
- Guardar segredo TOTP cifrado com chave fora do banco; segredos de serviços ficam no painel do provedor.
- Logs operacionais limitados, sem corpo de uploads ou mensagens. Auditoria administrativa com retenção inicial de 90 dias; tentativas de vínculo e desafios expirados com limpeza em 30 dias.
- Rodar limpeza ao iniciar e durante uso real, com locks e lotes limitados; suspensão da API pode atrasar a limpeza, sem prolongar validade lógica de tokens.
- Sem jobs que precisem de execução contínua para garantir segurança ou validade de desafios.
- Exportação manual semanal cifrada do banco e dos arquivos novos para equipamento já existente. Incluir todas as tabelas necessárias à restauração e manter segredos de configuração separadamente.
- Testar restauração antes do piloto; um dump PostgreSQL não inclui os objetos do Storage. Considerar transferência de backup nas cotas.
- Em pausa do provedor, mostrar manutenção e restaurar pelo painel quando possível; não simular perda definitiva nem prometer recuperação automática.

## 13. Organização prevista do código

```text
cmd/api/                    composição e inicialização do servidor
cmd/admin/                  bootstrap administrativo local
cmd/migrate/                execução explícita de migrações
internal/domain/            entidades, invariantes e transições puras
internal/application/       casos de uso e portas de saída
internal/adapters/httpapi/  adaptador de entrada HTTP e DTOs
internal/adapters/postgres/ persistência, transações e rate limit
internal/adapters/security/ Argon2id, tokens, TOTP e relógio
internal/adapters/webui/     arquivos estáticos e rotas do frontend
internal/config/            configuração validada no startup
internal/architecture/      testes das fronteiras de dependência
migrations/                 SQL versionado e runner transacional
openapi/                    contrato dos endpoints implementados
web/                        frontend React/TypeScript e testes de navegador
supabase/functions/         adaptador de webhook previsto para fase posterior
```

Dependências devem ser poucas, gratuitas e mantidas. Usar biblioteca auditável para Argon2id/TOTP e não criar algoritmo criptográfico próprio. Para testes de integração, PostgreSQL local em container, sem projetos de nuvem adicionais.

## 14. Configuração e implantação futura

### Variáveis principais

```dotenv
APP_ENV=production
PORT=10000
PUBLIC_BASE_URL=https://naosemetanobrasil.online
ALLOWED_ORIGINS=https://naosemetanobrasil.online
DATABASE_URL=<conexao-TLS-do-pooler-session>
DATABASE_MAX_CONNECTIONS=5
SUPABASE_URL=https://<projeto>.supabase.co
SUPABASE_SERVER_KEY=<segredo-somente-no-servidor>
SUPABASE_STORAGE_BUCKET=evidence-private
INSTAGRAM_OFFICIAL_HANDLE=naosemetanobrasil
INSTAGRAM_VERIFICATION_MODE=manual
VERIFICATION_CODE_PEPPER=<segredo-aleatorio>
TOTP_ENCRYPTION_KEY=<segredo-aleatorio>
UPLOADS_ENABLED=true
PUBLIC_WRITES_ENABLED=true
STORAGE_SOFT_LIMIT_BYTES=800000000
DATABASE_SOFT_LIMIT_BYTES=400000000
```

O Render fornece `PORT`; escutar em `0.0.0.0`. Validar configuração obrigatória no startup. Nunca preencher exemplos com tokens reais. A chave Supabase do servidor não pode aparecer em bundle público.

### Passos de implantação, após implementar o código

1. Criar uma organização e um projeto Supabase Free; confirmar cotas e ausência de add-ons.
2. Criar schema interno, usuário de aplicação e bucket privado; aplicar migrações a partir de máquina local.
3. Criar um Web Service Docker Free no Render sem forma de pagamento, usando o `Dockerfile` do projeto.
4. Configurar variáveis e segredos; manter apenas uma instância de API gratuita.
5. O Dockerfile faz o build React/TypeScript com Node e compila os comandos Go. Apenas arquivos estáticos e binários entram na imagem final; não há processo Node adicional em produção.
6. Start: entrypoint `/app/api`, com `FRONTEND_DIR=/app/web`. Migrações iniciais são executadas localmente; não depender de recurso pago de pre-deploy.
7. Configurar `/health/live` e validar os comportamentos de falha de banco separadamente.
8. Criar administrador, ativar TOTP e confirmar o perfil oficial que receberá directs.
9. Executar o fluxo completo com conta pessoal, inclusive conferência manual e retomada após suspensão da API.
10. Conferir no painel que nenhum recurso pago foi ativado; registrar limites e plano de backup.

Fonte para hospedagem Go: [Go no Render](https://render.com/docs/deploy-go-nethttp). Para fixar exatamente a versão do compilador na nuvem, usar Docker com imagem versionada; o runtime Go nativo do Render recebe atualizações. Fonte: [Linguagens e versões](https://render.com/docs/language-support).

## 15. Critérios de aceite da implementação

- [ ] Conta pessoal consegue vincular perfil por direct sem trocar seu tipo de conta.
- [ ] Vínculo exige mensagem realmente recebida e confirmação da sessão local correta.
- [ ] Códigos expirados, reutilizados, desconhecidos ou enviados por outro perfil não confirmam vínculo.
- [ ] Dois pedidos simultâneos não vinculam o mesmo perfil a usuários diferentes.
- [ ] Perfil de autoria e perfil afetado podem ser diferentes.
- [ ] Relato é enviado com categoria, descrição curta e rede; anexos são opcionais.
- [ ] Acompanhamento não é contado como testemunho ou evidência.
- [ ] Usuário não acessa nem altera rascunho, arquivo ou vínculo de outra pessoa.
- [ ] Arquivos pendentes não são públicos; retirada impede emissão de novas URLs de acesso.
- [ ] Limites de arquivo, armazenamento e escrita são aplicados inclusive sob concorrência.
- [ ] Dados sobrevivem a redeploy e suspensão do Render.
- [ ] Login local continua funcionando se o Instagram da pessoa ficar indisponível.
- [ ] Recuperação por código consome o código uma única vez e revoga sessões antigas.
- [ ] Administração exige papel e TOTP; decisões deixam auditoria mínima.
- [ ] Backup de banco e objetos é restaurado com sucesso em ambiente local.
- [ ] Render permanece Free sem pagamento; Supabase permanece Free sem add-ons; nenhum trial é obrigatório.
- [ ] Esgotamento simulado de cotas produz bloqueio controlado, sem upgrade ou cobrança automática.
- [ ] Na fase automática: assinatura inválida, entrega duplicada e API Go suspensa são tratados sem sucesso falso ou vínculo incorreto.

Validação técnica: `go test ./...`, `go test -race ./...` e `go vet ./...`, com testes de integração para transações de vínculo e recuperação de conta. Testes de autorização de arquivos acompanham a implementação futura de anexos. A primeira etapa está implementada; o restante deste documento define trabalho futuro. Nenhuma infraestrutura de nuvem foi provisionada.

## 16. Ordem de implementação

1. Estrutura Go, PostgreSQL local, migrações, autenticação e bootstrap administrativo.
2. Vínculo manual por direct, painel de conferência e confirmação final pelo usuário.
3. Rascunhos, envio de relatos, moderação e mural público.
4. Arquivos com limites, revisão de exposição pública e backup.
5. Participações, filtros, correções e controles de operação sem custo.
6. Piloto nos planos gratuitos, medindo armazenamento, transferência e trabalho de moderação.
7. Automação opcional do direct somente após permissões e validação técnica, mantendo fallback manual.

A prioridade é concluir o fluxo simples de relato e vínculo com custo zero. Crescimento deve respeitar as cotas: quando não houver capacidade gratuita, limitar novas operações ou pausar o piloto.
