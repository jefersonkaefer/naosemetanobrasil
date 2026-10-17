#!/usr/bin/env python3
"""Initial production setup. Credentials stay in memory and on /dev/tty."""
import argparse
import base64
import getpass
import os
from pathlib import Path
import re
import secrets
import shutil
import subprocess
import sys
import urllib.parse

ROOT = Path(__file__).resolve().parents[1]


def private_terminal():
    try:
        fd = os.open('/dev/tty', os.O_RDWR)
    except OSError:
        raise RuntimeError('Execute em um terminal privado interativo.') from None
    try:
        if not os.isatty(fd):
            raise RuntimeError('Terminal privado indisponível.')
        for target in (0, 1, 2):
            os.dup2(fd, target)
    finally:
        if fd > 2:
            os.close(fd)


def parse_pooler(raw):
    # The dashboard uses an unescaped [YOUR-PASSWORD] placeholder. Python
    # otherwise treats its brackets as an IPv6 literal in the authority.
    raw = raw.strip().replace('[YOUR-PASSWORD]', '%5BYOUR-PASSWORD%5D')
    try:
        url = urllib.parse.urlsplit(raw)
        port = url.port
    except ValueError:
        raise RuntimeError('URL inválida. Copie a conexão Session pooler do painel.') from None
    if (url.scheme not in ('postgres', 'postgresql')
            or not re.fullmatch(r'[a-z0-9-]+\.pooler\.supabase\.com', url.hostname or '')
            or port != 5432
            or url.path != '/postgres'
            or not re.fullmatch(r'postgres\.[a-z0-9]{20}', url.username or '')
            or url.fragment):
        raise RuntimeError('Use a URL Session pooler do Supabase, porta 5432, usuário postgres. Não informe a URL da Data API.')
    return url


def connection(url, username, password, ca):
    query = urllib.parse.urlencode({'sslmode': 'verify-full', 'sslrootcert': str(ca)})
    return urllib.parse.urlunsplit((
        'postgresql',
        urllib.parse.quote(username, safe='') + ':' + urllib.parse.quote(password, safe='')
        + '@' + url.hostname + ':5432',
        '/postgres', query, '',
    ))


def pg_environment(url, username, password, ca):
    env = {key: value for key, value in os.environ.items() if not key.startswith('PG')}
    return {**env, 'PGHOST': url.hostname, 'PGPORT': '5432', 'PGDATABASE': 'postgres',
            'PGUSER': username, 'PGPASSWORD': password, 'PGSSLMODE': 'verify-full',
            'PGSSLROOTCERT': str(ca), 'PGCONNECT_TIMEOUT': '10'}


def sql(psql, env, command):
    result = subprocess.run([psql, '-X', '-A', '-t', '-v', 'ON_ERROR_STOP=1', '-c', command],
                            env=env, capture_output=True, text=True, timeout=30)
    if result.returncode:
        raise RuntimeError('A conexão ou consulta ao banco falhou. Confira usuário, senha e certificado; detalhes sensíveis omitidos.')
    return result.stdout.strip()


def main():
    parser = argparse.ArgumentParser(description='Preparar um Supabase novo para Render, sem gravar segredos em arquivos.')
    parser.add_argument('--psql', default=shutil.which('psql'), help='Caminho do cliente psql instalado.')
    args = parser.parse_args()
    if not args.psql or not Path(args.psql).is_file():
        raise RuntimeError('Cliente psql indisponível. Instale-o ou informe --psql CAMINHO.')
    private_terminal()
    print('Preparação inicial de PRODUÇÃO. Nenhuma senha será gravada em arquivo ou passada como argumento.')
    raw = getpass.getpass('Cole a URL Session pooler do Supabase (oculta): ')
    url = parse_pooler(raw)
    password = urllib.parse.unquote(url.password or '')
    if not password or password.startswith('['):
        password = getpass.getpass('Senha do proprietário do banco: ')
    if not password or '\x00' in password:
        raise RuntimeError('Senha inválida.')
    ca = Path(input('Caminho do certificado raiz baixado do Supabase: ').strip()).expanduser().resolve()
    certificate = ca.read_bytes()
    if b'-----BEGIN CERTIFICATE-----' not in certificate or b'PRIVATE KEY' in certificate:
        raise RuntimeError('Informe somente o certificado raiz público, não uma chave privada.')
    owner_env = pg_environment(url, url.username, password, ca)
    sql(args.psql, owner_env, 'SELECT 1')
    migrate_env = {**os.environ, 'APP_ENV': 'production',
                   'DATABASE_URL': connection(url, url.username, password, ca)}
    if subprocess.run(['go', 'run', './cmd/migrate'], cwd=ROOT, env=migrate_env).returncode:
        raise RuntimeError('Migração não concluída.')
    if sql(args.psql, owner_env, 'SELECT count(*) FROM app.users') != '0':
        raise RuntimeError('Este comando é somente para um banco novo. Há contas existentes: reutilize os segredos atuais em vez de gerar novas chaves.')
    result = subprocess.run([args.psql, '-X', '-v', 'ON_ERROR_STOP=1', '-f',
                             str(ROOT / 'deploy/supabase-runtime.sql')],
                            env=owner_env, capture_output=True, timeout=30)
    if result.returncode:
        raise RuntimeError('Permissões do usuário nsm_api não aplicadas; detalhes sensíveis omitidos.')
    enabled = sql(args.psql, owner_env, "SELECT rolcanlogin FROM pg_roles WHERE rolname='nsm_api'") == 't'
    if not enabled:
        print('Agora defina uma senha independente para nsm_api. O psql solicitará a senha duas vezes.')
        if subprocess.run([args.psql, '-X', '-v', 'ON_ERROR_STOP=1', '-c', '\\password nsm_api'], env=owner_env).returncode:
            raise RuntimeError('Senha de nsm_api não configurada.')
        sql(args.psql, owner_env, 'ALTER ROLE nsm_api LOGIN')
    else:
        print('nsm_api já tem login. Sua senha existente será preservada.')
    runtime_password = getpass.getpass('Senha de nsm_api para validar e montar a conexão do Render: ')
    runtime_user = 'nsm_api.' + url.username.split('.', 1)[1]
    runtime_env = pg_environment(url, runtime_user, runtime_password, ca)
    if sql(args.psql, runtime_env, "SELECT current_user='nsm_api' AND has_table_privilege(current_user,'app.users','SELECT') AND NOT has_table_privilege(current_user,'app.nsm_migrations','SELECT')") != 't':
        raise RuntimeError('Permissões da conexão de runtime não conferem.')
    print('\nBanco e usuário conferidos. Guarde os três valores no gerenciador de senhas e copie para os campos privados do Render:')
    print('DATABASE_URL=' + connection(url, runtime_user, runtime_password, '/etc/secrets/supabase-ca.crt'))
    print('VERIFICATION_CODE_PEPPER=' + secrets.token_hex(32))
    print('TOTP_ENCRYPTION_KEY=' + base64.b64encode(secrets.token_bytes(32)).decode())
    print('\nCadastre também o certificado como Secret File supabase-ca.crt no Render. Não envie estes valores pelo chat.')


if __name__ == '__main__':
    try:
        main()
    except (Exception, KeyboardInterrupt) as error:
        if isinstance(error, RuntimeError):
            print(str(error), file=sys.stderr)
        else:
            print('Preparação interrompida; detalhes sensíveis omitidos. Nenhum segredo foi salvo em arquivo.', file=sys.stderr)
        sys.exit(1)
