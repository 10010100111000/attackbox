"""Verify the actual student evidence and the eight authored quiz answers."""
from collections import Counter
import configparser
import csv
from datetime import datetime, timezone
import json
from pathlib import Path
import re

root = Path(__file__).resolve().parents[1]
files = root / 'student-files'
with (files / 'files/jobs.csv').open(newline='') as stream:
    jobs = list(csv.DictReader(stream))
failed = [job for job in jobs if job['status'] == 'failed']
assert failed, 'missing failed jobs'
maximum = max(int(job['bytes']) for job in failed)
largest = [job for job in failed if int(job['bytes']) == maximum]
assert len(largest) == 1, 'largest failed job must be unambiguous'
settings = configparser.ConfigParser()
settings.read(files / 'files/settings.ini')
notes = list((files / 'hidden').rglob('.session-note'))
assert len(notes) == 1, 'hidden note must be unique'
case_code = re.findall(r'^case_code=(.+)$', notes[0].read_text(), re.MULTILINE)
assert len(case_code) == 1

pattern = re.compile(r'(?P<ip>\S+) - - \[(?P<time>[^]]+)\] "(?P<method>\S+) (?P<path>\S+) HTTP/1\.1" (?P<status>\d{3}) \d+ "-" "TrainingClient/1\.0"')
start = datetime(2026, 10, 20, 10, tzinfo=timezone.utc)
end = datetime(2026, 10, 20, 10, 10, tzinfo=timezone.utc)
records = []
for line in (files / 'logs/access.log').read_text().splitlines():
    match = pattern.fullmatch(line)
    assert match, 'invalid combined log record'
    record = match.groupdict()
    stamp = datetime.strptime(record['time'], '%d/%b/%Y:%H:%M:%S %z')
    if start <= stamp < end:
        records.append(record)
counts = Counter(record['ip'] for record in records if record['method'] == 'POST' and record['path'] == '/admin/login' and record['status'] in ('401', '403'))
ordered = counts.most_common()
assert len(ordered) >= 2 and ordered[0][1] > ordered[1][1], 'source ranking must have one winner'
source, count = ordered[0]
downloads = [record['path'] for record in records if record['ip'] == source and record['method'] == 'GET' and record['status'] == '200' and re.fullmatch(r'/backup/.*\.tar\.gz', record['path'])]
assert len(downloads) == 1, 'successful download must be unique'
actual = [str(len(failed)), largest[0]['job_id'], settings['production']['retention_days'], notes[0].relative_to(files / 'hidden').as_posix(), case_code[0], source, str(count), downloads[0]]

# The example intentionally uses simple scalar answers. Actual module parsing is
# verified by the platform parser separately; this check ties evidence to content.
source_md = (root / 'content/Linux 文件调查.md').read_text()
quizzes = re.findall(r'^```quiz\n(.*?)^```', source_md, re.MULTILINE | re.DOTALL)
authored = []
for quiz in quizzes:
    matches = re.findall(r'^answer: (.+)$', quiz, re.MULTILINE)
    assert len(matches) == 1
    value = matches[0]
    authored.append(json.loads(value) if value.startswith('"') else value)
assert len(authored) == 8
assert actual == authored, 'student evidence and course answers differ'
print('Verified 8 answers, unique evidence, log syntax and UTC window boundaries.')
