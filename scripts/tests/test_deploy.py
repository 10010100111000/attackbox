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
        self.assertEqual(d.ws_url('ws://worker.example.com:8443'), 'ws://worker.example.com:8443')
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

    def test_browser_gateway_addresses_and_http_setup(self):
        for origin, expected in [
            ('http://192.168.1.20', 'ws://192.168.1.20:8443'),
            ('http://203.0.113.20', 'ws://203.0.113.20:8443'),
            ('http://[fd00::20]:3000', 'ws://[fd00::20]:8443'),
            ('https://microsb.de', 'wss://microsb.de:8443'),
        ]:
            with self.subTest(origin=origin):
                d.ENV.unlink()
                args = d.parser().parse_args(['control','setup','--database','postgres://u:p@localhost/db','--origin',origin])
                d.setup_control(args)
                self.assertEqual(d.nodes(d.read_env())[0]['publicDataUrl'], expected)
        for value, expected in [('WSS://Worker.Example:443/', 'wss://worker.example'),
                                ('ws://[fd00:0::20]:8443/', 'ws://[fd00::20]:8443')]:
            self.assertEqual(d.ws_url(value), expected)
        for value in ['ws://0.0.0.0:8443','ws://[::]:8443','ws://worker.test:0','ws://worker.test?secret=x','ws://user:pass@worker.test']:
            with self.subTest(value=value), self.assertRaises((d.ConfigError, ValueError)):
                d.ws_url(value)
        values = d.read_env()
        values['SANDBOX_NODES_JSON'] = json.dumps([dict(id='local',label='Local',publicDataUrl='ws://127.0.0.1:8443',maxSlots=16)])
        with self.assertRaises(d.ConfigError):d.nodes(values)
        d.ENV.unlink()
        args = d.parser().parse_args(['control','setup','--database','postgres://u:p@localhost/db','--origin','https://microsb.de','--worker-public-url','ws://worker.test:8443'])
        with self.assertRaises(d.ConfigError):d.setup_control(args)
        self.assertFalse(d.ENV.exists())

    def test_https_setup_requires_certificate_or_explicit_proxy(self):
        values=d.read_env();items=d.nodes(values);items[0]['publicDataUrl']='wss://worker.example.test:9443'
        values['SANDBOX_NODES_JSON']=json.dumps(items);d.atomic(d.ENV,d.env_text(values))
        with self.assertRaises(d.ConfigError):d.setup_worker(d.parser().parse_args(['worker','setup']))
        self.assertFalse(d.WORKER.exists())
        d.setup_worker(d.parser().parse_args(['worker','setup','--tls-terminated','--listen-addr','127.0.0.1:9443']))
        self.assertFalse(d.yaml.safe_load(d.WORKER.read_text())['gateway']['tls_enabled'])

    def test_standalone_worker_requires_explicit_public_endpoint(self):
        d.ENV.unlink()
        args = d.parser().parse_args(['worker','setup','--control-url','wss://learn.example/cluster/connect','--cluster-secret','x' * 32])
        with self.assertRaises(d.ConfigError):d.setup_worker(args)
        self.assertFalse(d.WORKER.exists())
        args.public_url = 'ws://192.168.1.30:8443'
        d.setup_worker(args)
        self.assertEqual(d.yaml.safe_load(d.WORKER.read_text())['gateway']['public_data_url'], args.public_url)

    def cloudflare_values(self):
        values = d.read_env()
        cert = self.root/'origin.pem'
        key = self.root/'origin.key'
        cert.write_text('nginx -t validates the real certificate')
        key.write_text('test placeholder')
        items = d.nodes(values)
        items[0]['publicDataUrl'] = 'wss://microsb.de'
        values.update(CLOUDFLARE_ENABLED='true', APP_BASE_URL='https://microsb.de',
                      ORIGIN_TLS_CERT=str(cert), ORIGIN_TLS_KEY=str(key), SANDBOX_NODES_JSON=json.dumps(items))
        return values

    def test_cloudflare_headers_peer_allowlist_and_same_origin_websockets(self):
        values = self.cloudflare_values()
        config = d.render_nginx(values, '127.0.0.1:3000')
        self.assertIn('geo $realip_remote_addr $attackbox_cloudflare_peer', config)
        self.assertIn('real_ip_header CF-Connecting-IP;', config)
        self.assertIn('proxy_set_header X-Forwarded-For $remote_addr;', config)
        self.assertNotIn('$proxy_add_x_forwarded_for', config)
        self.assertNotIn('@@', config)
        self.assertIn('location = /ws/cli', config)
        self.assertIn('location = /ws/gui', config)
        self.assertIn('server 127.0.0.1:8443;', config)
        self.assertIn('ssl_reject_handshake on;', config)
        self.assertIn('location = /cluster/connect { return 404; }', config)

    def test_cloudflare_rejects_exposed_ports_wrong_origins_and_config_injection(self):
        values = self.cloudflare_values()
        for key, value in [('HOST','0.0.0.0'), ('WORKER_PROXY_UPSTREAM','0.0.0.0:8443'),
                           ('WORKER_PROXY_UPSTREAM','localhost:8443;bad'), ('ORIGIN_TLS_KEY','/tmp/a;bad'),
                           ('APP_BASE_URL','https://microsb.de:8443'), ('APP_BASE_URL','https://microsb.de/path'),
                           ('APP_BASE_URL','http://microsb.de'), ('CLOUDFLARE_ENABLED','yes'),
                           ('TURNSTILE_SITE_KEY','site-without-secret')]:
            with self.subTest(key=key, value=value), self.assertRaises((d.ConfigError, ValueError)):
                d.render_nginx({**values, key:value}, '127.0.0.1:3000')
        with self.assertRaises(d.ConfigError):
            d.render_nginx({**values, 'SANDBOX_NODES_JSON':d.read_env()['SANDBOX_NODES_JSON']}, '127.0.0.1:3000')

    def test_cloudflare_worker_must_be_loopback_without_native_tls(self):
        values = self.cloudflare_values()
        d.atomic(d.ENV, d.env_text(values))
        d.setup_worker(d.parser().parse_args(['worker','setup','--tls-terminated','--listen-addr','127.0.0.1:8443']))
        d.render_nginx(values, '127.0.0.1:3000')
        config = d.yaml.safe_load(d.WORKER.read_text())
        for updates in [dict(listen_addr='0.0.0.0:8443'), dict(tls_enabled=True), dict(listen_addr='127.0.0.1:9443')]:
            modified = {**config, 'gateway':{**config['gateway'], **updates}}
            d.atomic(d.WORKER, d.yaml.safe_dump(modified))
            with self.assertRaises(d.ConfigError):d.render_nginx(values, '127.0.0.1:3000')
        config['node']['control_url'] = 'wss://microsb.de/cluster/connect'
        d.atomic(d.WORKER, d.yaml.safe_dump(config))
        with self.assertRaises(d.ConfigError):d.render_nginx(values, '127.0.0.1:3000')

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
