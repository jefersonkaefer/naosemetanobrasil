# Publicação no Render Free

Repositório: [jefersonkaefer/naosemetanobrasil](https://github.com/jefersonkaefer/naosemetanobrasil), branch `main`. Domínio: `naosemetanobrasil.online`. Instagram: `@naosemetanobrasil`.

O Blueprint cria um Web Service Docker **Free**, com frontend e API juntos, na região **Virginia**. O banco fica no Supabase Free. O manifesto não cria banco Render, disco, worker ou recurso pago. Deploys automáticos estão desativados. Este documento prepara a publicação; não confirma que um serviço já foi publicado.

## 1. Criar o banco

1. Entrar no [Supabase Dashboard](https://supabase.com/dashboard), escolher uma organização **Free** e criar o projeto `naosemetanobrasil`.
2. Preferir **East US / North Virginia (`us-east-1`)** para ficar perto do serviço Render. Se indisponível, combinar as regiões antes de criar o serviço.
3. Definir a senha do banco no painel e guardá-la em um gerenciador de senhas. Não enviar por chat.
4. Aguardar o banco ficar disponível. Em **Connect**, copiar a conexão **Session pooler**, porta **5432**. Não usar 6543: o backend usa prepared statements.
5. Em **Database settings**, baixar o certificado raiz do banco e guardar fora do repositório. Usar `sslmode=verify-full` e `sslrootcert` na conexão.

Copiar o host e a referência do projeto do painel. O host não pode ser inferido pela região. O pooler de sessão funciona por IPv4 no Free. [Conexão Supabase](https://supabase.com/docs/guides/database/connecting-to-postgres).

## 2. Migrações e permissões

Em um terminal privado, configurar `APP_ENV=production` e `DATABASE_URL` com a conexão do proprietário, TLS e o caminho **local** do certificado. Não registrar a senha no histórico do shell. A API não carrega `.env` automaticamente.

```sh
rtk proxy go run ./cmd/migrate
```

Depois, com a conexão do proprietário nas variáveis `PGHOST`, `PGPORT`, `PGUSER`, `PGDATABASE`, `PGSSLMODE=verify-full` e `PGSSLROOTCERT`, executar os comandos abaixo. `-W` solicita a senha do proprietário sem colocá-la no comando:

```sh
rtk proxy psql -X -W -v ON_ERROR_STOP=1 -f deploy/supabase-runtime.sql
rtk proxy psql -X -W -c '\password nsm_api'
rtk proxy psql -X -W -v ON_ERROR_STOP=1 -c 'ALTER ROLE nsm_api LOGIN'
```

O script cria `nsm_api` sem login. O segundo comando solicita uma senha independente para esse usuário; o terceiro habilita o login. O script concede somente as operações usadas pela API, sem criação de tabelas, leitura do histórico de migrações ou leitura dos eventos de auditoria. Reexecutá-lo preserva a senha existente.

A `DATABASE_URL` do **Render** deve usar `nsm_api.PROJECT_REF` e sua senha, nunca o proprietário. O caminho do certificado no Render será diferente do local:

```text
postgresql://nsm_api.PROJECT_REF:SENHA_CODIFICADA@HOST_DO_POOLER:5432/postgres?sslmode=verify-full&sslrootcert=/etc/secrets/supabase-ca.crt
```

Esse é apenas um modelo: substituir os campos em um local privado e codificar os caracteres reservados da senha para URL. O schema `app` deve ficar fora dos schemas expostos pela Data API. O histórico fica em `app.nsm_migrations`.

As migrações são executadas com o proprietário, fora do build e da inicialização da API. O Render Free não oferece shell ou jobs avulsos para esse trabalho.

## 3. Segredos

Gerar `VERIFICATION_CODE_PEPPER` e `TOTP_ENCRYPTION_KEY` novos e independentes, conforme [README.md](README.md). O pepper deve ter pelo menos 32 bytes aleatórios; a chave TOTP deve representar 32 bytes aleatórios em Base64. Guardar no gerenciador de segredos junto da estratégia de backup.

Trocar o pepper invalida sessões, desafios e códigos de recuperação. Trocar a chave TOTP sem recifrar os registros impede o acesso administrativo. Não reutilizar as chaves locais, do Compose ou dos testes.

## 4. Serviço Render

1. No [Render Dashboard](https://dashboard.render.com), escolher **New → Blueprint** e conectar somente `jefersonkaefer/naosemetanobrasil`.
2. Se o repositório recriado não aparecer, usar **Public Git Repository** com `https://github.com/jefersonkaefer/naosemetanobrasil`, enquanto ele estiver público. Esse caminho dispensa ampliar permissões da integração GitHub; os deploys continuarão manuais. Para um repositório privado, autorizar somente esse repositório na integração. Escolher `main` e `render.yaml` da raiz.
3. Conferir um Web Service, runtime **Docker**, plano **Free**, região **Virginia**.
4. Preencher nos campos privados `DATABASE_URL`, `VERIFICATION_CODE_PEPPER` e `TOTP_ENCRYPTION_KEY`. Não colocar valores no YAML, no GitHub ou no chat.
5. Em **Environment → Secret Files**, cadastrar `supabase-ca.crt` com o certificado raiz do Supabase. Ele estará em `/etc/secrets/supabase-ca.crt`. O Docker usa um usuário sem privilégios de root e o grupo necessário para ler o arquivo.
6. Fazer deploy manual após salvar a configuração completa. Se o Blueprint disparar um primeiro build antes de cadastrar o certificado, concluir a configuração e repetir o deploy manual.
7. Conferir o build e o estado **Live**. O health check `/health/ready` verifica conexão, schema e permissões do banco.

O primeiro deploy usa `RENDER_EXTERNAL_URL`, fornecido pelo Render. Cadastro e login funcionam no endereço HTTPS `onrender.com` exibido pelo painel, antes de configurar DNS. Testar `/health/ready`, cadastro, login e logout nesse endereço.

As contas da prévia local não são copiadas automaticamente. Relatos públicos e uploads ainda são prévias, conforme o README.

O Dockerfile não consome segredos no build. Certificados, chaves, arquivos de ambiente e dumps estão excluídos do contexto Docker e do Git. [Secret files no Render](https://render.com/docs/docker-secrets).

## 5. Domínio e administrador

O Blueprint declara `naosemetanobrasil.online`. Em **Settings → Custom Domains**, consultar os registros DNS exigidos e aplicá-los no registrador. Aguardar a validação e o certificado HTTPS. Depois, definir no serviço e fazer deploy:

```text
PUBLIC_BASE_URL=https://naosemetanobrasil.online
```

Essa origem explícita substitui o endereço automático. Usar o domínio final para autenticação; mutações no `onrender.com` serão rejeitadas por origem. O Blueprint não sobrescreve essa variável.

Criar o administrador em um terminal privado com as variáveis **de produção** e o certificado **local** configurados:

```sh
rtk proxy go run ./cmd/admin -username seu_usuario
# Para promover uma conta já cadastrada no site publicado:
rtk proxy go run ./cmd/admin -promote -username seu_usuario
```

O comando exige senha e autenticador, usa o terminal privado mesmo sob RTK e não envia esses dados ao chat. A promoção mantém a conta e encerra as sessões antigas. Entrar com **Acesso administrativo** e o próximo código do autenticador. Não existe administrador padrão; `jefkaefer` da prévia local não é migrada automaticamente.

## Limites e estado

Manter ambos os planos Free e evitar método de pagamento no Render para impedir cobranças de excedentes. As cotas podem suspender o serviço ou novos builds. O serviço pode dormir após 15 minutos sem tráfego. O PostgreSQL Free do Render expira em 30 dias, por isso não é utilizado. [Limites oficiais](https://render.com/docs/free).

O primeiro deploy usa `INSTAGRAM_VERIFICATION_MODE=manual`. A automação Meta não bloqueia a publicação. Para habilitar depois, seguir [INSTAGRAM_INTEGRATION.md](INSTAGRAM_INTEGRATION.md). Callback final: `https://naosemetanobrasil.online/v1/webhooks/instagram`.

As imagens Node e Go foram confirmadas no registro oficial. O build completo da imagem ainda deve ser validado pelo Render: o daemon Docker local não respondeu. Preparação local não confirma um deploy na nuvem. Conferir as permissões reais do Supabase e as pendências em [SECURITY_REVIEW.md](SECURITY_REVIEW.md).

Referências: [Blueprints](https://render.com/docs/blueprint-spec), [variáveis automáticas](https://render.com/docs/environment-variables), [conexão Supabase](https://supabase.com/docs/guides/database/connecting-to-postgres).
