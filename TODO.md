# GoAway - Hoja de Ruta y TODO

Ultima actualizacion: 2026-11-24

Este archivo solo contiene trabajo pendiente. Lo completado esta en [DONE.md](DONE.md).

Tablero recomendado (GitHub Projects): Inbox, Ready, In Progress, Review, Blocked, Done.

Labels base: epic, feature, enhancement, security, dns, dhcp, ha, frontend, api, observability, migration, breaking-change, good-first-issue, p0, p1, p2.

---

## Epicas abiertas

| Epic | Objetivo | Prioridad | Estado |
| --- | --- | ---: | --- |
| EPIC-11 Paridad Final & v1.0.0 | Soak/benchmarks, cobertura Vitest y release v1.0.0 | P1 | En curso (solo falta cierre v1.0.0) |

---

## P1 - Paridad Final (Blocky) y Preparacion v1.0.0

### EPIC-11 Paridad Final & v1.0.0

- [ ] TECH: Endurecimiento, soak tests y release v1.0.0
  - Prioridad: p1
  - Labels: enhancement
  - Contexto: todas las epicas de v0.9 a v1.2 estan completas en `v0.96.0`; falta validar estabilidad bajo carga sostenida antes de cortar `v1.0.0`.
  - Criterios de aceptacion:
    - Benchmarks/soak tests en UDP, TCP, DoT, DoH y DoQ sin fugas de goroutines ni memoria.
    - Cobertura de componentes clave en Vitest (`client/`).
    - Guia de migracion y corte oficial de `v1.0.0`.

---

## P0 - Deuda que bloquea lo demas

(Sin bloqueantes P0 pendientes; ver [DONE.md](DONE.md).)

---

## Seguimiento de la revision del 2026-11-24

Hallazgos resueltos (ver [DONE.md](DONE.md)).

---

## Seguimiento de la revision del 2026-09-07

Hallazgos resueltos (ver [DONE.md](DONE.md)).

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
