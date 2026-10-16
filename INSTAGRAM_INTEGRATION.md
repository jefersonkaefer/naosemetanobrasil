# Vínculo automático pelo Instagram

## Fluxo implementado

1. A pessoa informa o perfil e recebe um código único e temporário na plataforma.
2. Copia somente o código e o envia para `@naosemetanobrasil` pelo Instagram que quer vincular. Não precisa tornar a própria conta profissional.
3. A Meta entrega a mensagem ao webhook. O backend valida a assinatura, a conta destinatária e a identidade do remetente pela Graph API.
4. Se código, prazo e perfil coincidirem, o desafio passa a `awaiting_confirmation`.
5. A pessoa volta ao site, atualiza o status e confirma o perfil exibido. O retorno do foco à janela também atualiza o status; não há polling contínuo.

Não enviamos um segundo código ou mensagens automáticas. O botão “Já enviei” apenas informa o envio: não comprova a identidade. O código é exibido uma única vez e deve ser enviado como o texto completo da mensagem, sem comentários ou mais de um código.

## Ativação no aplicativo Meta e no Render

A conta **do projeto** precisa ser profissional (empresa ou criador) e ter acesso à API de mensagens. Usamos Instagram API com Instagram Login; esse fluxo não exige vincular uma Página Facebook. A conta da pessoa que envia o código pode continuar pessoal.

1. No Meta for Developers, configurar um aplicativo para Instagram API com Instagram Login e conectar `@naosemetanobrasil`.
2. Configurar os acessos `instagram_business_basic` e `instagram_business_manage_messages`. Confirmar no painel o nível de acesso e a revisão necessários para atender pessoas fora da equipe de teste; teste com papel de desenvolvedor não prova liberação ao público.
3. Obter o ID numérico da conta profissional conectada e o token de acesso correspondente. Não usar o `@nome` como ID. Guardar o token e o App Secret no painel privado do Render.
4. Gerar um token independente e aleatório de pelo menos 32 caracteres para a validação de inscrição do webhook.
5. Aplicar as migrações, incluindo `002_instagram_webhook.sql`. Se o papel da API for diferente do proprietário das tabelas, conceder a ele `SELECT/INSERT` em `app.instagram_message_receipts`, além das permissões existentes; concessões anteriores a novas tabelas não se estendem automaticamente sem privilégios padrão. Configurar no servidor:

   | Variável | Valor |
   | --- | --- |
   | `INSTAGRAM_OFFICIAL_HANDLE` | `naosemetanobrasil` |
   | `INSTAGRAM_VERIFICATION_MODE` | `webhook` |
   | `INSTAGRAM_ACCOUNT_ID` | ID numérico da conta profissional conectada |
   | `INSTAGRAM_ACCESS_TOKEN` | Token privado da conta profissional |
   | `META_APP_SECRET` | Segredo privado do aplicativo Meta |
   | `META_WEBHOOK_VERIFY_TOKEN` | Token aleatório definido no passo 4 |
   | `META_GRAPH_API_VERSION` | Versão suportada selecionada no painel Meta, no formato `vN.N` |

   A versão é explícita: não usamos `latest` nem fazemos downgrade automático. O endpoint de consulta de remetente é `GET https://graph.instagram.com/{versão}/{IGSID}?fields=id,username`, autenticado por Bearer no servidor.

6. Cadastrar a callback HTTPS `https://naosemetanobrasil.online/v1/webhooks/instagram` no painel Meta com o mesmo verify token. O backend responde ao GET de inscrição com `hub.challenge` somente quando token e modo são válidos.
7. Assinar o campo `messages` para a conta profissional conectada. Conferir no painel que a assinatura é da conta correta, além de configurar a URL do aplicativo.
8. Enviar um código real pelo Instagram de uma conta autorizada para teste e confirmar o vínculo no site. Depois, testar com uma pessoa fora da equipe quando o acesso público estiver liberado. Renovar o token conforme seu ciclo de validade; não manter token expirado em produção.

O endpoint retorna 404 no modo `manual`. No modo `webhook`, a API só inicializa com a configuração completa. O administrador ainda pode fazer conferência manual explícita pelos endpoints protegidos, sem se passar pelo webhook.

## Segurança e entregas repetidas

- Cada POST exige `X-Hub-Signature-256`, validado com HMAC-SHA256 sobre os bytes originais e comparação em tempo constante. O verify token do GET não autentica mensagens.
- `object`, `entry.id` e `recipient.id` precisam corresponder ao Instagram configurado. Ecos, exclusões, eventos sem mensagem e IDs inválidos são ignorados.
- A identidade vem do ID de remetente assinado e de uma consulta autenticada à Graph API. O nome escrito no texto ou no formulário não substitui essa consulta.
- Código inválido, desconhecido, cancelado, expirado ou enviado por outro perfil não avança o vínculo. Mensagens anteriores à criação do desafio e timestamps futuros inválidos também não servem como prova.
- Recebimento, mudança de estado, recibo e auditoria são gravados na mesma transação. A chave `(account_id, message_id)` deduplica as entregas, inclusive concorrentes.
- A confirmação final é feita pelo dono autenticado, com CSRF. O banco impede dois vínculos ativos para a mesma identidade remota, mesmo que o perfil mude de nome.
- Texto da mensagem, código em claro e tokens Meta não são gravados na tabela de recibos ou nos logs. IDs remotos permanecem privados no banco e não aparecem na resposta pública do perfil.
- O handler processa a entrega dentro da requisição com prazo de oito segundos e chamadas externas limitadas a quatro segundos. Só responde sucesso após o commit. Falhas transitórias retornam 503 para permitir nova entrega; não há goroutine desacoplada ou fila em memória que se perca ao dormir no Render.
- O Render Free pode dormir. Isso pode atrasar a entrega. Não há promessa de recebimento instantâneo nem garantia de recuperação após o fim das tentativas da Meta; a pessoa pode atualizar o status, reenviar o mesmo código enquanto válido ou usar a conferência manual.

## Validação e limites atuais

Testes unitários cobrem assinaturas, filtros de eventos, limites de corpo, inscrição, consulta de perfil, falhas de API e regras do desafio. Testes com PostgreSQL cobrem recebimento assinado, repetição, concorrência, cancelamento, expiração e unicidade por ID remoto. O E2E cobre geração de código, webhook assinado, confirmação no navegador e persistência do método `instagram_webhook`.

Nos testes, somente a identidade consultada na Meta é simulada. A API Go e o PostgreSQL são reais. A implementação sintética é compilada exclusivamente com a tag `e2e`, exige banco local `_test` e credencial sintética; não está no binário Docker de produção. Testes não enviam mensagens nem usam credenciais reais da Meta.

Pendente: configurar o aplicativo e as credenciais, publicar a callback HTTPS e testar a entrega real. Retenção e exclusão dos recibos devem acompanhar a política de desafios/auditoria antes de abrir o piloto público.

Referências: [workspace oficial Instagram da Meta](https://www.postman.com/meta/instagram/overview), [Instagram Login](https://www.postman.com/meta/instagram/folder/6raa77c/instagram-api-with-instagram-login), [webhooks Meta](https://developers.facebook.com/docs/graph-api/webhooks/getting-started), [perfil do remetente](https://developers.facebook.com/docs/instagram-platform/instagram-api-with-instagram-login/messaging-api/user-profile).
