import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


def load(name):
    spec = importlib.util.spec_from_file_location(name, ROOT / 'scripts' / f'{name}.py')
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


setup, manage = load('setup'), load('manage')


class SetupTest(unittest.TestCase):
    def test_generation_roundtrip_and_no_overwrite(self):
        # Characters must survive systemd-compatible quoting without shell expansion.
        database = 'postgres://user:p%40ss@127.0.0.1/db'
        values, node = setup.configuration(database, 'http://localhost', 're_$secret', 'Demo <sender@example.com>')
        keys = [values[k] for k in ['SANDBOX_CLUSTER_SECRET', 'SANDBOX_SIGNING_SEED_HEX', 'CONTENT_CREDENTIALS_KEY', 'QUIZ_ANSWER_HMAC_SECRET']]
        self.assertEqual(len(set(keys)), 4)
        for key in keys:
            self.assertEqual(len(bytes.fromhex(key)), 32)
        self.assertIn(values['SANDBOX_CLUSTER_SECRET'], node)
        self.assertIn(json.loads(values['SANDBOX_NODES_JSON'])[0]['publicDataUrl'], node)
        with tempfile.TemporaryDirectory() as directory:
            env, yaml = Path(directory) / 'web.env', Path(directory) / 'node.yaml'
            setup.write_configuration(env, yaml, values, node)
            self.assertEqual(manage.read_environment(env), values)
            self.assertEqual(env.stat().st_mode & 0o777, 0o600)
            self.assertEqual(yaml.stat().st_mode & 0o777, 0o600)
            original = env.read_bytes(), yaml.read_bytes()
            with self.assertRaises(ValueError):
                setup.write_configuration(env, yaml, *setup.configuration(database, 'http://localhost'))
            self.assertEqual(original, (env.read_bytes(), yaml.read_bytes()))
            yaml.unlink()
            with self.assertRaises(ValueError):
                setup.write_configuration(env, yaml, values, node)
            self.assertFalse(yaml.exists())

    def test_validation_and_https(self):
        for database, origin in [('invalid', 'http://localhost'), ('postgres://u:p@localhost/db', 'http://example.com'), ('postgres://u:p@localhost/db', 'https://example.com/path')]:
            with self.assertRaises(ValueError):
                setup.configuration(database, origin)
        values, node = setup.configuration('postgres://u:p@localhost/db', 'https://example.com')
        self.assertIn('wss://example.com:8443', node)
        self.assertEqual(values['RESEND_API_KEY'], '')
        with self.assertRaises(ValueError):
            setup.configuration('postgres://u:p@localhost/db', 'http://localhost', 'key', '')


if __name__ == '__main__':
    unittest.main()
