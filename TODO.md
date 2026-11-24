# GoAway - Hoja de Ruta y TODO

Ultima actualizacion: 2026-11-24

Este archivo solo contiene trabajo pendiente. Lo completado esta en [DONE.md](DONE.md).

Tablero recomendado (GitHub Projects): Inbox, Ready, In Progress, Review, Blocked, Done.

Labels base: epic, feature, enhancement, security, dns, dhcp, ha, frontend, api, observability, migration, breaking-change, good-first-issue, p0, p1, p2.

---

## Epicas abiertas

| Epic | Objetivo | Prioridad | Estado |
| --- | --- | ---: | --- |
| EPIC-11 Paridad Final & v1.0.0 | Soak/benchmarks, cobertura Vitest y release v1.0.0 | P1 | En curso (las tres tareas sin empezar) |

---

## P0 - Deuda que bloquea lo demas

- [ ] SECURITY: Rotar el `jwtSecret` filtrado
  - Prioridad: p0
  - Labels: security
  - Contexto: `settings.yaml` ya esta fuera del repo (ver DONE.md), pero el secreto que contenia sigue en el historial de git y no se ha rotado.
  - Criterios de aceptacion:
    - Nuevo `jwtSecret` generado en todos los despliegues; los tokens antiguos dejan de validar.
    - Decidido si se purga el historial (`git filter-repo`) o se asume el secreto como quemado.

---

## P1 - Paridad Final (Blocky) y Preparacion v1.0.0

### EPIC-11 Paridad Final & v1.0.0

Umbrales propuestos (revisar antes de empezar).

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
