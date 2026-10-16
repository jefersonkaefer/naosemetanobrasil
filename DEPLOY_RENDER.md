# Publicação no Render Free

O responsável já criou a conta no Render, registrou `naosemetanobrasil.online` e criou o Instagram `@naosemetanobrasil`. Este arquivo prepara a publicação; nenhum serviço foi criado ou publicado por esta alteração.

## Serviço

O `render.yaml` define um único Web Service Docker no plano `free`, com frontend e API na mesma origem. O banco permanece externo, no Supabase do projeto. O manifesto não cria banco Render, disco, worker ou recurso pago. Deploys automáticos estão desativados.

1. Disponibilizar este projeto em um repositório Git acessível à conta do Render. O checkout local ainda não tem repositório Git configurado.
2. Criar ou usar o PostgreSQL do projeto no Supabase Free. Guardar a URL do pooler de sessão, com usuário e senha e `sslmode=verify-full`, nas variáveis do provedor. Não enviar credenciais por chat nem versioná-las.
3. Gerar os dois segredos independentes seguindo o README. Guardar o pepper e a chave TOTP em um gerenciador de segredos e usá-los no serviço; não trocar depois da publicação sem migração dos dados afetados.
4. Aplicar as migrações a partir de um ambiente local confiável, com `APP_ENV=production` e `DATABASE_URL` já definidos no processo:

   ```sh
   rtk proxy go run ./cmd/migrate
   ```

   O comando usa a URL do ambiente e não carrega `.env` automaticamente. A migração é versionada e pode ser executada novamente. Não depende de shell, job avulso ou comando de pre-deploy do Render Free.

5. No Render, importar o repositório por **New → Blueprint**, usando `render.yaml`. Informar `DATABASE_URL`, `VERIFICATION_CODE_PEPPER` e `TOTP_ENCRYPTION_KEY` nos campos privados. Confirmar que o serviço está no plano **Free**.
6. No painel do serviço, consultar os registros DNS solicitados para `naosemetanobrasil.online` e aplicá-los no registrador do domínio. Esperar a validação do domínio e do certificado HTTPS.
7. Abrir `https://naosemetanobrasil.online/health/ready` e conferir cadastro e login pelo domínio final. A configuração de origem do backend é esse domínio: testar mutações pelo subdomínio `onrender.com` dará erro de origem. Se usar temporariamente o subdomínio Render, alterar `PUBLIC_BASE_URL` para a origem exata exibida pelo painel e restaurar o domínio final depois.
8. Criar o administrador a partir de um ambiente local confiável, com a configuração de produção no processo, usando `rtk proxy go run ./cmd/admin -username administrador`. Para promover uma conta existente, usar `rtk proxy go run ./cmd/admin -promote -username seu_usuario`: exige a senha atual e TOTP, preserva a conta e encerra as sessões antigas. Os comandos solicitam a senha no terminal privado. Não há administrador padrão nem senha sintética em produção.

Aplicar a migração 003 e usar credencial de runtime diferente da credencial de migração. O histórico fica em `app.nsm_migrations` e não deve receber grants para a API. Conferir as permissões reais no Supabase e as pendências registradas em [SECURITY_REVIEW.md](SECURITY_REVIEW.md).

O build Docker ainda precisa ser validado no Render; a configuração local anterior foi testada com os binários e PostgreSQL, sem um daemon Docker disponível.

## Limites do plano gratuito

O serviço gratuito pode dormir após 15 minutos sem tráfego e levar cerca de um minuto para voltar. Isso pode atrasar ou provocar novas entregas de webhooks. Uma integração de mensagens deverá tolerar novas entregas, persistir eventos e não depender de memória ou arquivos locais.

Manter a conta sem método de pagamento e acompanhar as cotas. O Render documenta suspensão de serviços ou builds quando os limites são atingidos sem pagamento cadastrado. O PostgreSQL gratuito do Render expira em 30 dias, por isso não é usado neste manifesto.

Referências: [Blueprints](https://render.com/docs/blueprint-spec), [limites gratuitos](https://render.com/docs/free).

## Automação do Instagram

O recebimento automático do código está implementado e documentado em [INSTAGRAM_INTEGRATION.md](INSTAGRAM_INTEGRATION.md). A pessoa copia o código da plataforma, envia pelo próprio Instagram e confirma o vínculo no site após a conferência automática. Não há segundo código nem mensagem de resposta automática.

O Blueprint começa com `INSTAGRAM_VERIFICATION_MODE=manual` para continuar funcionando sem credenciais Meta. Após aplicar a migração `002_instagram_webhook.sql`, configurar no painel privado do Render `META_APP_SECRET`, `META_WEBHOOK_VERIFY_TOKEN`, `INSTAGRAM_ACCESS_TOKEN`, `INSTAGRAM_ACCOUNT_ID` e `META_GRAPH_API_VERSION`. Só então alterar o modo para `webhook` no Blueprint e no serviço, para que um futuro sync não restaure `manual`, e cadastrar a URL HTTPS no aplicativo Meta. Credenciais incompletas no modo webhook impedem a inicialização; não há fallback silencioso.

O webhook do domínio final será `https://naosemetanobrasil.online/v1/webhooks/instagram`. Não publicar tokens no repositório nem compartilhar por chat. A confirmação real com a Meta e a publicação no Render ainda estão pendentes.
