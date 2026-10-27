import importlib.util
import json
import os
from pathlib import Path
import tempfile
import types
import unittest
from unittest.mock import patch

ROOT=Path(__file__).resolve().parents[2]
spec=importlib.util.spec_from_file_location('deployment', ROOT/'scripts/deploy.py')
d=importlib.util.module_from_spec(spec)
spec.loader.exec_module(d)


class DeploymentTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory()
        self.root=Path(self.temp.name)
        self.patches=[]
        for key,value in {'ROOT':self.root,'ENV':self.root/'etc/attackbox/web.env','WORKER':self.root/'etc/attackbox/worker.yaml','ETC':self.root/'etc','WEB':self.root/'www/attackbox','BIN':self.root/'bin/attackbox-worker'}.items():
            p=patch.object(d,key,value);p.start();self.patches.append(p)
        for role in ['control','worker']:
            (self.root/'deploy'/role).mkdir(parents=True)
            for p in (ROOT/'deploy'/role).iterdir():
                if p.is_file():
                    (self.root/'deploy'/role/p.name).write_bytes(p.read_bytes())
        self.args=d.parser().parse_args(['control','setup','--database','postgres://u:p%40ss@localhost/db','--origin','http://localhost'])
        d.setup_control(self.args)

    def tearDown(self):
        for p in reversed(self.patches):p.stop()
        self.temp.cleanup()

    def test_secrets_roundtrip_and_no_overwrite(self):
        values=d.read_env()
        keys=[values[k] for k in ['SANDBOX_CLUSTER_SECRET','SANDBOX_SIGNING_SEED_HEX','CONTENT_CREDENTIALS_KEY','QUIZ_ANSWER_HMAC_SECRET']]
        self.assertEqual(len(set(keys)),4)
        self.assertEqual(d.ENV.stat().st_mode & 0o777,0o600)
        original=d.ENV.read_bytes()
        with self.assertRaises(d.ConfigError):d.setup_control(self.args)
        self.assertEqual(d.ENV.read_bytes(),original)
        values['MAIL_FROM']='Demo "Name" <sender@example.test>'
        values['RESEND_API_KEY']='$literal\\value'
        d.atomic(d.ENV,d.env_text(values))
        self.assertEqual(d.read_env(),values)

    def test_ipv6_tls_and_node_contract(self):
        self.assertEqual(d.endpoint('::',3300),'[::1]:3300')
        self.assertEqual(d.endpoint('0.0.0.0',3000),'127.0.0.1:3000')
        with self.assertRaises(ValueError):d.endpoint('127.0.0.1','3000;bad')
        with self.assertRaises(d.ConfigError):d.ws_url('ws://worker.example.com:8443')
        args=d.parser().parse_args(['worker','setup'])
        d.setup_worker(args)
        config=d.yaml.safe_load(d.WORKER.read_text())
        d.match_worker(d.read_env(),config)
        self.assertEqual(d.worker_health(config),'http://127.0.0.1:8443/health')
        config['gateway'].update(tls_enabled=True,public_data_url='wss://worker.example.test:9443')
        self.assertEqual(d.worker_health(config),'https://worker.example.test:9443/health')
        with self.assertRaises(d.ConfigError):d.match_worker(d.read_env(),config)
        d.node_add(d.parser().parse_args(['control','node','add','--node-id','remote','--public-url','wss://worker.example.test:9443']))
        config['node']['node_id']='remote';d.match_worker(d.read_env(),config)
        self.assertEqual(len(d.nodes(d.read_env())),2)

    def test_https_setup_requires_certificate_or_explicit_proxy(self):
        values=d.read_env();items=d.nodes(values);items[0]['publicDataUrl']='wss://worker.example.test:9443'
        values['SANDBOX_NODES_JSON']=json.dumps(items);d.atomic(d.ENV,d.env_text(values))
        with self.assertRaises(d.ConfigError):d.setup_worker(d.parser().parse_args(['worker','setup']))
        self.assertFalse(d.WORKER.exists())
        d.setup_worker(d.parser().parse_args(['worker','setup','--tls-terminated','--listen-addr','127.0.0.1:9443']))
        self.assertFalse(d.yaml.safe_load(d.WORKER.read_text())['gateway']['tls_enabled'])

    def prepare_publish(self):
        artifact=self.root/'web/.output/server';artifact.mkdir(parents=True)
        (artifact/'index.mjs').write_text('new')
        os.chmod(artifact,0o700);os.chmod(artifact/'index.mjs',0o600)
        (d.WEB/'.output/server').mkdir(parents=True)
        (d.WEB/'.output/server/index.mjs').write_text('old')
        config=d.ETC/'nginx/sites-available/attackbox';d.atomic(config,'old nginx',0o644)
        (d.ETC/'nginx/sites-enabled').mkdir(parents=True)
        (d.ETC/'nginx/sites-enabled/default').symlink_to('/old/default')
        unit=d.ETC/'systemd/system/attackbox-web.service';d.atomic(unit,'old unit',0o644)
        return config,unit

    def test_failed_health_restores_release_and_configuration(self):
        config,unit=self.prepare_publish()
        with patch.object(d,'prepare_account'),patch.object(d,'run') as run,patch.object(d.subprocess,'run',return_value=types.SimpleNamespace(returncode=0)),patch.object(d,'health',side_effect=RuntimeError('unhealthy')):
            with self.assertRaises(RuntimeError):d.deploy_control()
            commands=[tuple(c.args) for c in run.call_args_list]
            self.assertIn(('systemctl','restart','attackbox-web'),commands)
        self.assertEqual((d.WEB/'.output/server/index.mjs').read_text(),'old')
        self.assertEqual(config.read_text(),'old nginx');self.assertEqual(unit.read_text(),'old unit')
        self.assertTrue((d.ETC/'nginx/sites-enabled/default').is_symlink())
        self.assertFalse(list(d.WEB.parent.glob('.attackbox-stage-*')))

    def test_success_keeps_previous_release_and_starts_nginx(self):
        self.prepare_publish()
        with patch.object(d,'prepare_account'),patch.object(d,'run') as run,patch.object(d.subprocess,'run',return_value=types.SimpleNamespace(returncode=1)),patch.object(d,'health'):
            d.deploy_control()
            self.assertIn(('systemctl','start','nginx'),[tuple(c.args) for c in run.call_args_list])
        self.assertEqual((d.WEB/'.output/server/index.mjs').read_text(),'new')
        self.assertEqual((d.WEB.with_name(d.WEB.name+'.prev')/'.output/server/index.mjs').read_text(),'old')
        self.assertEqual((d.WEB/'.output/server').stat().st_mode & 0o777,0o755)
        self.assertEqual((d.WEB/'.output/server/index.mjs').stat().st_mode & 0o777,0o644)

    def test_worker_failed_health_restores_binary(self):
        d.setup_worker(d.parser().parse_args(['worker','setup']))
        config=d.yaml.safe_load(d.WORKER.read_text())
        config['runtime']['instances_dir']=str(self.root/'instances')
        config['compose_labs']['state_dir']=str(self.root/'labs')
        d.atomic(d.WORKER,d.yaml.safe_dump(config))
        d.BIN.parent.mkdir();d.BIN.write_text('old')
        (self.root/'dist').mkdir();(self.root/'dist/attackbox-worker').write_text('new')
        unit=d.ETC/'systemd/system/attackbox-worker.service';d.atomic(unit,'old unit',0o644)
        with patch.object(d,'run'),patch.object(d,'health',side_effect=RuntimeError('unhealthy')):
            with self.assertRaises(RuntimeError):d.deploy_worker()
        self.assertEqual(d.BIN.read_text(),'old');self.assertEqual(unit.read_text(),'old unit')

    def test_nginx_validation_failure_preserves_running_release(self):
        config,unit=self.prepare_publish()
        def command(*args,**kwargs):
            if args==('nginx','-t'):
                raise RuntimeError('invalid nginx')
        with patch.object(d,'prepare_account'),patch.object(d,'run',side_effect=command),patch.object(d.subprocess,'run',return_value=types.SimpleNamespace(returncode=0)):
            with self.assertRaises(RuntimeError):d.deploy_control()
        self.assertEqual((d.WEB/'.output/server/index.mjs').read_text(),'old')
        self.assertEqual(config.read_text(),'old nginx')
        self.assertEqual(unit.read_text(),'old unit')

    def test_secret_file_and_duplicate_env(self):
        secret=self.root/'password';secret.write_text('not-in-process-arguments');os.chmod(secret,0o600)
        args=d.parser().parse_args(['control','admin','--email','admin@example.test','--password-file',str(secret)])
        d.secret_files(args);self.assertEqual(args.password,'not-in-process-arguments')
        args.password=None;os.chmod(secret,0o644)
        with self.assertRaises(d.ConfigError):d.secret_files(args)
        original=d.ENV.read_text();d.ENV.write_text(original+'PORT=4000\n')
        with self.assertRaises(d.ConfigError):d.read_env()



if __name__=='__main__':unittest.main()
