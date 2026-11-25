# GoAway - Hoja de Ruta y TODO

Ultima actualizacion: 2026-11-25

Este archivo solo contiene trabajo pendiente. Lo completado esta en [DONE.md](DONE.md).

Tablero recomendado (GitHub Projects): Inbox, Ready, In Progress, Review, Blocked, Done.

Labels base: epic, feature, enhancement, security, dns, dhcp, ha, frontend, api, observability, migration, breaking-change, good-first-issue, p0, p1, p2.

---

## Epicas abiertas

| Epic | Objetivo | Prioridad | Estado |
| --- | --- | ---: | --- |
| EPIC-11 Paridad Final & v1.0.0 | Soak/benchmarks y release v1.0.0 | P1 | En curso (falta soak y el tag) |

Orden sugerido: soak y por ultimo release.

---

## P0 - Deuda que bloquea lo demas

- [ ] SECURITY: Decidir si se purga del historial el `jwtSecret` filtrado
  - Prioridad: p0
  - Labels: security
  - Contexto: la rotacion ya es automatica (ver DONE.md): una instancia que arranca con el secreto filtrado genera uno nuevo. Queda decidir que hacer con el historial de git.
  - Criterios de aceptacion:
    - Decidido si se purga el historial (`git filter-repo` + force-push, rompe clones y forks) o se asume el secreto como quemado (recomendado: ya no da acceso a ninguna instancia actualizada).

---

## P1 - Paridad Final (Blocky) y Preparacion v1.0.0

### EPIC-11 Paridad Final & v1.0.0

Umbrales propuestos (revisar antes de empezar).

- [ ] DEPS: TypeScript 7 (bloqueado)
  - Prioridad: p2
  - Labels: frontend, enhancement
  - Contexto: resto de la actualizacion de dependencias para v1.0.0 hecha (ver DONE.md). `typescript` 6.0.3 -> 7.0.2 rompe ESLint: `typescript-eslint` (latest 8.74.0 y canary) solo admite `typescript >=4.8.4 <6.1.0`.
  - Criterios de aceptacion:
    - Actualizar cuando `typescript-eslint` publique soporte para TS 7; `pnpm -C client exec eslint`, build y Vitest en verde.
    - Opcional: `tsc -b` tiene ~36 errores de tipos preexistentes (CI no lo ejecuta); limpiarlos y anadir `tsc -b` al job `frontend`.

- [ ] TECH: Benchmarks y soak tests del resolver
  - Prioridad: p1
  - Labels: enhancement, dns
  - Contexto: no existe ningun `Benchmark*` ni test de fugas en el repo.
  - Criterios de aceptacion:
    - `Benchmark*` para UDP, TCP, DoT, DoH y DoQ (y DoH3 si esta habilitado).
    - `goleak.VerifyNone` en los tests de arranque/parada de cada servidor.
    - Soak de 10 min a 1000 QPS por protocolo: RSS estable (crecimiento < 10% tras 2 min de calentamiento) y numero de goroutines estable.
    - Resultados documentados como baseline en `docs/`.

- [ ] RELEASE: Corte de v1.0.0
  - Prioridad: p1
  - Labels: migration
  - Contexto: guia de migracion (`docs/docs/migration-1.0.md`), entrada de CHANGELOG y versiones a 1.0.0 ya hechas (ver DONE.md).
  - Criterios de aceptacion:
    - Decidido si las referencias al upstream (`pommee/goaway`) pasan al fork: updater/installer (`backend/updater/updater.go`, `installer.sh`, `updater.sh`), comprobacion de version (`server-statistics.tsx`, `nav-actions.tsx`), changelog en la UI (`changelog.tsx`), `docs/docs/install.md` y README.
    - Tag `v1.0.0` (o antes `v1.0.0-rc.1`): binarios GoReleaser en GitHub Releases e imagen multi-arch en `ghcr.io/rasty94/goaway:1.0.0`.

---

## P2 - Mejoras menores

_(sin tareas)_

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
