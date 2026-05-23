#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 gmail|forwardmail" >&2
  exit 2
}

mode="${1:-}"
case "$mode" in
  gmail|forwardmail) ;;
  *) usage ;;
esac

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

set -a
source .env
set +a

case "$mode" in
  gmail)
    smtp_host="${LIVE_TEST_GMAIL_SMTP_HOST:?}"
    smtp_port="${LIVE_TEST_GMAIL_SMTP_PORT:?}"
    smtp_user="${LIVE_TEST_GMAIL_SMTP_USER:?}"
    smtp_password="${LIVE_TEST_GMAIL_SMTP_PASSWORD:?}"
    from_email="${LIVE_TEST_GMAIL_FROM_EMAIL:?}"
    from_name="${LIVE_TEST_GMAIL_FROM_NAME:?}"
    recipient="${LIVE_TEST_GMAIL_RECIPIENT:?}"
    ;;
  forwardmail)
    smtp_host="${LIVE_TEST_FORWARDMAIL_SMTP_HOST:?}"
    smtp_port="${LIVE_TEST_FORWARDMAIL_SMTP_PORT:?}"
    smtp_user="${LIVE_TEST_FORWARDMAIL_SMTP_USER:?}"
    smtp_password="${LIVE_TEST_FORWARDMAIL_SMTP_PASSWORD:?}"
    from_email="${LIVE_TEST_FORWARDMAIL_FROM_EMAIL:?}"
    from_name="${LIVE_TEST_FORWARDMAIL_FROM_NAME:?}"
    recipient="${LIVE_TEST_FORWARDMAIL_RECIPIENT:?}"
    ;;
esac

api="${LIVE_TEST_API_BASE:?}"
vps="${LIVE_TEST_VPS_HOST:-vps}"
worker_image="${LIVE_TEST_WORKER_IMAGE:?}"
suffix="$(date +%s)"
email="kleos-${mode}-live-${suffix}@abhiyadav.in"
password="Kleos-${mode}-${suffix}-pass"

signup="$(
  jq -n --arg email "$email" --arg password "$password" --arg name "Kleos ${mode} Live Test" \
    '{email:$email,password:$password,name:$name}' |
    curl -fsS -H 'Content-Type: application/json' -d @- "$api/auth/signup"
)"
access="$(jq -r '.access' <<<"$signup")"
user_id="$(jq -r '.user.id' <<<"$signup")"

smtp_payload="$(
  jq -n \
    --arg label "$mode live test" \
    --arg host "$smtp_host" \
    --argjson port "$smtp_port" \
    --arg username "$smtp_user" \
    --arg password "$smtp_password" \
    --arg from_email "$from_email" \
    --arg from_name "$from_name" \
    '{label:$label,host:$host,port:$port,username:$username,password:$password,from_email:$from_email,from_name:$from_name,use_tls:true}'
)"
smtp_resp="$(
  curl -fsS -H 'Content-Type: application/json' -H "Authorization: Bearer $access" \
    -d "$smtp_payload" "$api/smtp"
)"
smtp_id="$(jq -r '.smtp.id' <<<"$smtp_resp")"

verify_resp="$(
  curl -fsS -H 'Content-Type: application/json' -H "Authorization: Bearer $access" \
    -d '{}' "$api/smtp/$smtp_id/verify"
)"
if [[ "$(jq -r '.ok' <<<"$verify_resp")" != "true" ]]; then
  jq -c '{verify:.}' <<<"$verify_resp" >&2
  exit 1
fi

match_line="$(
  ssh "$vps" "cd /opt/kleos && sudo docker compose -f deploy/docker-compose.yml exec -T postgres psql -U kleos -d kleos -v ON_ERROR_STOP=1 -At -v user_id='$user_id' -v smtp_id='$smtp_id' -v recipient='$recipient' -v mode='$mode'" <<'SQL'
WITH input AS (
  SELECT :'user_id'::uuid AS user_id, :'smtp_id'::uuid AS smtp_id, :'recipient'::text AS recipient, :'mode'::text AS mode
), resume_row AS (
  INSERT INTO resumes (user_id, filename, storage_path, parsed_text, is_active)
  SELECT user_id, 'kleos-' || mode || '-live-test.txt', '/tmp/kleos-' || mode || '-live-test.txt',
         'Kleos Live Test
Backend engineer with Go, PostgreSQL, Docker, and SMTP delivery experience.', true
  FROM input
  RETURNING id, user_id
), campaign_row AS (
  INSERT INTO campaigns (user_id, name, resume_id, smtp_id)
  SELECT i.user_id, 'Live ' || i.mode || ' sender self-test', r.id, i.smtp_id
  FROM input i JOIN resume_row r ON true
  RETURNING id, user_id
), company_row AS (
  INSERT INTO companies (name, slug, domain)
  SELECT 'Kleos ' || mode || ' Self Test',
         'kleos-' || mode || '-self-test-' || extract(epoch from now())::bigint,
         'abhiyadav.in'
  FROM input
  RETURNING id
), job_row AS (
  INSERT INTO jobs (source, external_id, company_id, title, description, location, remote, url, raw)
  SELECT 'manual-' || i.mode || '-live-test',
         'kleos-' || i.mode || '-live-' || extract(epoch from now())::bigint,
         c.id, 'Backend Engineer',
         'Build Go services with PostgreSQL and Docker for a production outreach system.',
         'Remote', true, 'https://abhiyadav.in/kleos/', '{}'::jsonb
  FROM input i JOIN company_row c ON true
  RETURNING id, company_id
), recruiter_row AS (
  INSERT INTO recruiters (company_id, email, name, title, source, confidence, evidence_url)
  SELECT j.company_id, i.recipient, 'Abhinav', 'Self Test Recipient',
         'manual-' || i.mode || '-live-test', 'high', 'https://abhiyadav.in/kleos/'
  FROM input i JOIN job_row j ON true
  RETURNING id, email
), match_row AS (
  INSERT INTO campaign_matches (campaign_id, job_id, match_score, state)
  SELECT c.id, j.id, 999, 'generated'
  FROM campaign_row c JOIN job_row j ON true
  RETURNING id, campaign_id, job_id
), draft_row AS (
  INSERT INTO email_drafts (match_id, recruiter_id, variant, subject, body_text, chosen, spam_score)
  SELECT m.id, r.id, 1,
         'Live ' || i.mode || ' sender verification',
         'Hi Abhinav,

This is a controlled Kleos live sender verification using ' || i.mode || ' SMTP. It is addressed only to ' || i.recipient || ' and exercises the deployed sender path, warmup gating, sent_emails persistence, and match state transition.

Best,
Kleos',
         true, 0.01
  FROM input i JOIN match_row m ON true JOIN recruiter_row r ON true
  RETURNING id, match_id
)
SELECT match_row.id::text || ' ' || draft_row.id::text || ' ' || (SELECT recipient FROM input)
FROM match_row JOIN draft_row ON draft_row.match_id = match_row.id;
SQL
)"
match_id="$(awk '{print $1}' <<<"$match_line")"

queue_line="$(
  ssh "$vps" "cd /opt/kleos && sudo docker compose -f deploy/docker-compose.yml exec -T postgres psql -U kleos -d kleos -v ON_ERROR_STOP=1 -At -v recipient='$recipient'" <<'SQL'
SELECT count(*)
FROM campaign_matches m
JOIN campaigns c ON c.id=m.campaign_id
JOIN email_drafts d ON d.match_id=m.id AND d.chosen=true
JOIN recruiters r ON r.id=d.recruiter_id
WHERE m.state='generated' AND c.status='active' AND r.email <> :'recipient';
SQL
)"
if [[ "$queue_line" != "0" ]]; then
  echo "refusing to run sender: found generated rows for a different recipient" >&2
  exit 1
fi

ssh "$vps" "sudo docker run --rm --env-file /opt/kleos/.env --network kleos_default -v /opt/kleos/bin:/kleosbin:ro --entrypoint /kleosbin/worker-sender '$worker_image' --limit 1 --skip-jitter"

result="$(
  ssh "$vps" "cd /opt/kleos && sudo docker compose -f deploy/docker-compose.yml exec -T postgres psql -U kleos -d kleos -v ON_ERROR_STOP=1 -At -v match_id='$match_id'" <<'SQL'
SELECT 'state=' || m.state || ' recipient=' || s.recruiter_email::text ||
       ' status=' || s.status || ' response=' || COALESCE(s.smtp_response,'') ||
       ' message_id=' || s.message_id
FROM campaign_matches m
JOIN sent_emails s ON s.match_id=m.id
WHERE m.id=:'match_id'::uuid;
SQL
)"
echo "$result"
if [[ "$result" != state=sent* || "$result" != *" status=sent "* ]]; then
  exit 1
fi
