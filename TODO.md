# GoAway - Hoja de Ruta y TODO

Ultima actualizacion: 2026-11-24

Este archivo solo contiene trabajo pendiente. Lo completado esta en [DONE.md](DONE.md).

Tablero recomendado (GitHub Projects): Inbox, Ready, In Progress, Review, Blocked, Done.

Labels base: epic, feature, enhancement, security, dns, dhcp, ha, frontend, api, observability, migration, breaking-change, good-first-issue, p0, p1, p2.

---

## Epicas abiertas

| Epic | Objetivo | Prioridad | Estado |
| --- | --- | ---: | --- |
| EPIC-12 CI/CD GitHub | Que el pipeline corra de verdad, valide `main` y publique releases | P0 | Codigo hecho; falta configuracion manual en GitHub |
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

Estado a 2026-11-24: `rasty94/goaway` tiene 0 runs y `main` sin proteccion. Los runs que aparecen en `gh run list` sin `-R` son del remote `upstream` (`pommee/goaway`); usar `gh ... -R rasty94/goaway`.

- [ ] OPS (manual, en GitHub): Activar Actions y configurar el repo
  - Prioridad: p0
  - Labels: ci
  - Contexto: el codigo de EPIC-12 esta hecho (ver DONE.md); falta la parte que solo se puede hacer en la web/admin de GitHub. Releases por tag usan `GITHUB_TOKEN`, no hacen falta secrets propios.
  - Criterios de aceptacion:
    - Averiguado por que no se disparan los workflows (pestana Actions / billing) y un push a `main` genera run del workflow `CI`.
    - Settings > Actions > Workflow permissions permite a `GITHUB_TOKEN` escribir packages (GHCR) y contents (GitHub Release).
    - Branch protection en `main` con checks obligatorios: `frontend`, `backend`, `e2e`, `helm`, `terraform-provider`.
    - Primer tag de prueba publica binarios y la imagen `ghcr.io/rasty94/goaway` (o se valida con el corte de v1.0.0).
    - Borrar secrets obsoletos si existieran (`GH_TOKEN`, `DOCKER_USERNAME`, `DOCKER_PASSWORD`).

---

## P1 - Paridad Final (Blocky) y Preparacion v1.0.0

### EPIC-11 Paridad Final & v1.0.0

Umbrales propuestos (revisar antes de empezar).

- [ ] DEPS: Actualizacion completa de dependencias para v1.0.0
  - Prioridad: p1
  - Labels: enhancement, security, breaking-change
  - Contexto: inventario a 2026-11-24 (`go list -m -u`, `pnpm outdated`).
  - Toolchain e imagenes:
    - Go `1.26.5` -> `1.27.x` (`go.mod`, `Dockerfile.dev`, `Dockerfile.multi`, `docker-compose.e2e.yml`; los workflows leen `go.mod`).
    - Node `22` -> LTS actual (`Dockerfile.multi`, `ci.yml`, `release.yml`); pnpm `10.23.0` fijado en ambos workflows.
    - `alpine:3.23` -> ultima estable (`Dockerfile`, `Dockerfile.multi`).
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
