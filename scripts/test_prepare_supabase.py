import importlib.util
import io
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
import urllib.parse
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('prepare_supabase', Path(__file__).with_name('prepare-supabase.py'))
setup = importlib.util.module_from_spec(spec)
spec.loader.exec_module(setup)

URL = 'postgresql://postgres.abcdefghijklmnopqrst:synthetic-owner@aws-0-us-east-1.pooler.supabase.com:5432/postgres'


class ProductionSetupSafety(unittest.TestCase):
    def test_accepts_dashboard_password_placeholder(self):
        parsed = setup.parse_pooler(URL.replace('synthetic-owner', '[YOUR-PASSWORD]'))
        self.assertEqual(urllib.parse.unquote(parsed.password), '[YOUR-PASSWORD]')

    def test_refuses_other_hosts_and_transaction_pooling(self):
        for raw in (URL.replace('5432', '6543'), URL.replace('.pooler.supabase.com', '.pooler.supabase.com.attacker.test'), URL.replace('postgres.abcdefghijklmnopqrst', 'nsm_api.abcdefghijklmnopqrst')):
            with self.subTest(raw=raw), self.assertRaises(RuntimeError):
                setup.parse_pooler(raw)

    def test_encodes_password_and_enforces_verified_tls(self):
        parsed = setup.parse_pooler(URL + '?sslmode=disable&host=elsewhere')
        result = urllib.parse.urlsplit(setup.connection(parsed, 'nsm_api.abcdefghijklmnopqrst', 'synthetic:@/?#', '/etc/secrets/supabase-ca.crt'))
        self.assertEqual(urllib.parse.unquote(result.password), 'synthetic:@/?#')
        self.assertEqual(urllib.parse.parse_qs(result.query), {'sslmode': ['verify-full'], 'sslrootcert': ['/etc/secrets/supabase-ca.crt']})
        with patch.dict(os.environ, {'PGSERVICE': 'untrusted', 'PGSSLMODE': 'disable'}):
            env = setup.pg_environment(parsed, 'nsm_api.abcdefghijklmnopqrst', 'synthetic', '/tmp/root.crt')
        self.assertNotIn('PGSERVICE', env)
        self.assertEqual(env['PGSSLMODE'], 'verify-full')

    def test_existing_accounts_prevent_role_changes_and_new_keys(self):
        with tempfile.TemporaryDirectory() as temp:
            ca = Path(temp) / 'root.crt'
            ca.write_text('-----BEGIN CERTIFICATE-----\nsynthetic-test-only\n-----END CERTIFICATE-----')
            with patch.object(sys, 'argv', ['prepare-supabase.py', '--psql', sys.executable]), \
                    patch.object(setup, 'private_terminal'), \
                    patch.object(setup.getpass, 'getpass', return_value=URL), \
                    patch('builtins.input', return_value=str(ca)), \
                    patch.object(setup, 'sql', side_effect=['1', '1']), \
                    patch.object(setup.subprocess, 'run', return_value=subprocess.CompletedProcess([], 0)) as run, \
                    patch.object(sys, 'stdout', io.StringIO()), \
                    patch.object(setup.secrets, 'token_bytes') as generate:
                with self.assertRaisesRegex(RuntimeError, 'contas existentes'):
                    setup.main()
                self.assertEqual(run.call_count, 1)
                self.assertEqual(run.call_args.args[0], ['go', 'run', './cmd/migrate'])
                generate.assert_not_called()


if __name__ == '__main__':
    unittest.main()
