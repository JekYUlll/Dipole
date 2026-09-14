set positional-arguments

env_file := env('DIPOLE_ENV_FILE', '.env')
project := env('COMPOSE_PROJECT_NAME', 'dipole')
compose := 'docker compose --env-file "' + env_file + '" --project-name "' + project + '" -f deploy/compose/docker-compose.microservices.yml'
experience := compose + ' -f deploy/microservices/agent-experience.yml --profile search'

default:
    @just --list

# Build graph lives in Makefile; accepts targets such as images or image-core.
build target='build':
    make "$1"

install:
    npm --prefix services/agent-runtime ci
    npm --prefix frontend ci

certs:
    test -f certs/internal/ca.pem || scripts/generate-internal-certs.sh

up:
    {{compose}} up -d --wait

agent-up:
    {{experience}} up -d --build --wait

# Uses the same project for both IM and Agent; preserves data volumes.
down:
    {{experience}} down

logs service='agent':
    {{experience}} logs --tail 100 -f "$1"

web:
    DIPOLE_WEB_PROXY_TARGET=http://127.0.0.1:8080 npm --prefix frontend run dev

check:
    scripts/check-go.sh
    scripts/check-sqlc.sh
    scripts/check-proto.sh
    scripts/check-compose.sh
    scripts/check-service-layout.sh

test-agent:
    npm --prefix services/agent-runtime run typecheck
    npm --prefix services/agent-runtime test

# Live smoke creates demo data and restarts this project's Agent worker.
smoke:
    COMPOSE_PROJECT_NAME="{{project}}" node scripts/smoke-agent-experience.mjs
