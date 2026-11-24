# GoAway - Hoja de Ruta y TODO

Ultima actualizacion: 2026-11-24

Este archivo solo contiene trabajo pendiente. Lo completado esta en [DONE.md](DONE.md).

Tablero recomendado (GitHub Projects): Inbox, Ready, In Progress, Review, Blocked, Done.

Labels base: epic, feature, enhancement, security, dns, dhcp, ha, frontend, api, observability, migration, breaking-change, good-first-issue, p0, p1, p2.

---

## Epicas abiertas

| Epic | Objetivo | Prioridad | Estado |
| --- | --- | ---: | --- |
| EPIC-12 CI/CD GitHub | Que el pipeline corra de verdad, valide `main` y publique releases | P0 | Sin empezar (0 runs en `rasty94/goaway`) |
| EPIC-11 Paridad Final & v1.0.0 | Dependencias al dia, soak/benchmarks, cobertura Vitest y release v1.0.0 | P1 | En curso (tareas sin empezar) |

Orden sugerido: EPIC-12 primero (sin CI no se valida nada de lo demas), luego actualizacion de dependencias, y despues soak/Vitest/release.

---

## P0 - Deuda que bloquea lo demas

- [ ] SECURITY: Rotar el `jwtSecret` filtrado
  - Prioridad: p0
  - Labels: security
  - Contexto: `settings.yaml` ya esta fuera del repo (ver DONE.md), pero el secreto que contenia sigue en el historial de git y no se ha rotado.
  - Criterios de aceptacion:
    - Nuevo `jwtSecret` generado en todos los despliegues; los tokens antiguos dejan de validar.
    - Decidido si se purga el historial (`git filter-repo`) o se asume el secreto como quemado.

### EPIC-12 CI/CD GitHub

Estado a 2026-11-24: los 3 workflows estan `active` pero `rasty94/goaway` tiene 0 runs, 0 secrets y `main` sin proteccion. Los runs que aparecen en `gh run list` sin `-R` son del remote `upstream` (`pommee/goaway`).

- [ ] OPS (manual, en GitHub): Activar Actions y configurar el repo
  - Prioridad: p0
  - Labels: ci
  - Criterios de aceptacion:
    - Averiguado por que no se disparan los workflows (pestana Actions / billing) y un push a `main` genera run.
    - Branch protection en `main`: checks obligatorios del workflow de validacion.
    - Secrets necesarios creados segun la estrategia de release elegida (ver abajo).

- [ ] DECISION: Estrategia de release
  - Prioridad: p0
  - Labels: ci
  - Contexto: v0.95/v0.96 se hicieron a mano (`chore(release): prepare ...` + tag), pero `release.yml` usa semantic-release, que tambien escribe CHANGELOG y tags. Son incompatibles.
  - Opciones: (a) semantic-release automatico en cada push a `main`; (b) release manual, workflow disparado por tag `v*`.
  - Criterio: una sola via documentada en `CONTRIBUTING`/README; la otra eliminada.

- [ ] CI: Validacion tambien en push a `main`
  - Prioridad: p0
  - Labels: ci
  - Contexto: `pull-request.yml` solo corre en PRs y se esta commiteando directo a `main`, asi que ningun commit reciente ha pasado lint/tests/E2E.
  - Criterios de aceptacion:
    - Workflow de validacion con `on: push: branches: [main]` ademas de `pull_request` (commitlint solo en PR).
    - Cache de Go modules y pnpm store.

- [ ] CI: Release condicionada a tests y publicacion correcta
  - Prioridad: p0
  - Labels: ci
  - Contexto: `release.yml` publica sin pasar tests; hace login en DockerHub pero `make publish` empuja a `ghcr.io/rasty94/goaway`; instala goreleaser con `curl https://git.io/goreleaser | bash` (git.io retirado y script remoto sin verificar).
  - Criterios de aceptacion:
    - Release depende del job de validacion (`needs:` o `workflow_run` con exito).
    - `docker/login-action` con `registry: ghcr.io`, `GITHUB_TOKEN` y `permissions: packages: write`.
    - `goreleaser/goreleaser-action` con version fijada.
    - Chart `version`/`appVersion` sincronizados con el tag.

- [ ] CI: Fijar versiones y endurecer workflows
  - Prioridad: p1
  - Labels: ci, security
  - Criterios de aceptacion:
    - Sin `latest`: pnpm (usar `packageManager` de `client/package.json`), `govulncheck`, goreleaser.
    - Actions fijadas por SHA (o al menos major actual) y `permissions` minimas por job.
    - `golangci-lint` sobre el modulo raiz (hoy `working-directory: ./backend` sin `go.mod` ni `.golangci.yml`); anadir `.golangci.yml` con los linters elegidos.
    - `timeout-minutes` en todos los jobs (en upstream hubo runs colgados 720h).

- [ ] CI: Cubrir Helm chart y Terraform provider
  - Prioridad: p1
  - Labels: ci
  - Criterios de aceptacion:
    - `helm lint` + `helm template` con `values.yaml` y `values-stateless.yaml`.
    - Job para `terraform-provider-goaway/` (modulo Go propio): `go vet`, `go test`, build.

- [ ] CI: Actualizaciones automaticas de dependencias
  - Prioridad: p1
  - Labels: ci
  - Criterios de aceptacion:
    - `.github/dependabot.yml` (o Renovate) para gomod (raiz y provider), npm (`client/`), docker y github-actions, agrupado semanalmente.

---

## P1 - Paridad Final (Blocky) y Preparacion v1.0.0

### EPIC-11 Paridad Final & v1.0.0

Umbrales propuestos (revisar antes de empezar).

- [ ] DEPS: Actualizacion completa de dependencias para v1.0.0
  - Prioridad: p1
  - Labels: enhancement, security, breaking-change
  - Contexto: inventario a 2026-11-24 (`go list -m -u`, `pnpm outdated`).
  - Toolchain e imagenes:
    - Go `1.26.5` -> `1.27.x` (`go.mod`, `Dockerfile.dev`, `Dockerfile.multi`, workflows).
    - Node `22` -> LTS actual (`Dockerfile.multi`, workflows).
    - `alpine:3.23` -> ultima estable (`Dockerfile`, `Dockerfile.multi`).
    - GitHub Actions a sus ultimas majors (`checkout`, `setup-node`, `setup-python`, `cache`, `pnpm/action-setup`, `docker/login-action`, `golangci-lint-action`).
  - Go (11 directas desactualizadas, todas minor/patch o pre-1.0):
    - `codeberg.org/miekg/dns` v0.6.85 -> v0.6.143 (pre-1.0, revisar API: es el nucleo del resolver).
    - `quic-go/quic-go` v0.59.1 -> v0.66.0 (pre-1.0, afecta DoQ/DoH3).
    - `minio/minio-go/v7` v7.0.97 -> v7.4.1, `prometheus/client_golang` v1.23.2 -> v1.25.0, `pro-bing` v0.8.0 -> v0.9.1, `go-toml/v2` v2.2.4 -> v2.4.3.
    - Resto patch: `gin-contrib/cors`, `gin-contrib/gzip`, `insomniacslk/dhcp`, `gopsutil/v4`, `gorm`.
    - `go get -u ./... && go mod tidy` tambien en `terraform-provider-goaway/` (hoy sin pendientes directas; revisar indirectas).
  - Frontend (`client/`, 35 desactualizadas: 24 minor/patch, 11 major):
    - Majors: `typescript` 6 -> 7, `@tanstack/react-table` 8 -> 9, `i18next` 25 -> 27 + `react-i18next` 16 -> 18, `motion` 12 -> 14, `@tsparticles/*` 3 -> 4, `@commitlint/*` 20 -> 21.
    - Minor/patch: react, react-dom, react-router-dom, vite, tailwindcss, eslint, radix-ui, recharts, etc.
  - Criterios de aceptacion:
    - Una PR (o commit) por grupo: toolchain, Go, frontend minor, cada frontend major; cada uno con CI verde.
    - Majors con notas de migracion revisadas (changelog upstream) y cambios de API aplicados.
    - `govulncheck` y `pnpm audit --prod` sin hallazgos high/critical.
    - Tests Go (`-race`), Vitest y E2E en verde tras el ultimo grupo.
    - Hacer despues de EPIC-12 para que cada salto quede validado por CI.

- [ ] TECH: Benchmarks y soak tests del resolver
  - Prioridad: p1
  - Labels: enhancement, dns
  - Contexto: no existe ningun `Benchmark*` ni test de fugas en el repo.
  - Criterios de aceptacion:
    - `Benchmark*` para UDP, TCP, DoT, DoH y DoQ (y DoH3 si esta habilitado).
    - `goleak.VerifyNone` en los tests de arranque/parada de cada servidor.
    - Soak de 10 min a 1000 QPS por protocolo: RSS estable (crecimiento < 10% tras 2 min de calentamiento) y numero de goroutines estable.
    - Resultados documentados como baseline en `docs/`.

- [ ] TECH: Cobertura Vitest de componentes clave
  - Prioridad: p1
  - Labels: frontend
  - Contexto: hoy solo hay `client/src/lib/utils.test.ts` y `client/src/pages/validation.test.ts`, sin tests de componentes.
  - Criterios de aceptacion:
    - Tests de login (incl. OIDC/2FA), settings, listas (blacklist/whitelist) y query log.
    - Cobertura >= 60% de lineas en `client/src/` y `npm test` en CI.

- [ ] RELEASE: Guia de migracion y corte de v1.0.0
  - Prioridad: p1
  - Labels: migration
  - Criterios de aceptacion:
    - Guia de migracion 0.9x -> 1.0.0 en `docs/` (cambios de config, `schemaVersion`, rollback).
    - CHANGELOG, tag y release `v1.0.0`.

---

## P2 - Mejoras menores

- [ ] BUG: Tamano de BD en modo stateless
  - Prioridad: p2
  - Labels: api
  - Contexto: `getDBSizeMB` (`backend/api/server.go`) lee `data/database.db`, que no existe con `misc.stateless: true`.
  - Criterios de aceptacion:
    - En stateless devuelve el tamano de la BD en memoria (`PRAGMA page_count * page_size`) o 0 sin error.

---

## Milestones

- [x] v0.9 Resolver: EPIC-01 + base de EPIC-03.
- [x] v1.0 Policy: EPIC-02 + resto de EPIC-03 + privacidad base.
- [x] v1.1 Cluster: EPIC-05 (auth, outbox persistente, resync logico, reloj logico, nombres estables, mTLS), importador Pi-hole y API keys con scopes.
- [x] v1.2 Enterprise: auth avanzada (OIDC, LDAP/AD, TOTP 2FA, WebAuthn/passkeys) + consistencia dual-stack + ecosistema IaC (Helm chart + Terraform provider) + paridad resolver (DoQ, parallel_best, EDE, qtype, Redis, exportador JSON/CSV/Postgres).

Nota: el CHANGELOG va por 0.96.0 mientras que v0.9, v1.0, v1.1 y v1.2 ya estan cubiertos funcionalmente. Tras cerrar EPIC-11, listo para saltar a **v1.0.0**.

---

## Definition of Done (Transversal)

- Tests unitarios.
- Tests de integracion o E2E.
- Metricas y logs necesarios.
- Documentacion actualizada.
- Changelog actualizado.
- Plan de migracion y rollback.
- Mover la tarea de este fichero a DONE.md.

## Plantilla Minima de Issue

Goal, User story, Scope, Out of scope, API/Config changes, UI changes, Metrics/Logs, Security impact, Migration/Upgrade impact, Acceptance criteria, Test plan.
