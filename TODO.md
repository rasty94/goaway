# GoAway - Hoja de Ruta y TODO

Ultima actualizacion: 2026-11-24

Este archivo solo contiene trabajo pendiente. Lo completado esta en [DONE.md](DONE.md).

Tablero recomendado (GitHub Projects): Inbox, Ready, In Progress, Review, Blocked, Done.

Labels base: epic, feature, enhancement, security, dns, dhcp, ha, frontend, api, observability, migration, breaking-change, good-first-issue, p0, p1, p2.

---

## Epicas abiertas

| Epic | Objetivo | Prioridad | Estado |
| --- | --- | ---: | --- |
| EPIC-06 Ecosystem | Provider Terraform oficial | P1 | API keys, Pi-hole import, GitOps y Helm hechos |
| EPIC-07 Enterprise Auth | LDAP/AD, passkeys | P2 | OIDC hecho |
| EPIC-09 Resolver Avanzado | Sink SQL externo (PostgreSQL/TimescaleDB) | P1 | DoQ, parallel_best, EDE, qtype, Redis y export JSON/CSV hechos |

---

## P0 - Deuda que bloquea lo demas

(Sin bloqueantes P0 pendientes; ver [DONE.md](DONE.md).)

---

## Seguimiento de la revision del 2026-11-24

Hallazgos resueltos (ver [DONE.md](DONE.md)).

---

## Seguimiento de la revision del 2026-09-07

Hallazgos resueltos (ver [DONE.md](DONE.md)).

## P1 - Operacion, Resiliencia y Ecosistema

### EPIC-06 Ecosystem

- [ ] FEATURE: Provider Terraform oficial
  - Prioridad: p1
  - Labels: feature, api
  - Criterios de aceptacion:
    - Recursos de provider para upstreams, listas, grupos, politicas.
    - Ejemplos verificados en CI.

### EPIC-09 Resolver Avanzado (Paridad DNS / Blocky)

- [ ] FEATURE: Sink SQL externo para query logs (PostgreSQL/TimescaleDB)
  - Prioridad: p2
  - Labels: feature, observability
  - Contexto: la cache compartida Redis y el exportador estructurado JSON/CSV/stdout ya estan implementados.
  - Criterios de aceptacion:
    - Exportador opcional a PostgreSQL/TimescaleDB.

---

## P2 - Enterprise y Dual Stack Completo

### EPIC-07 Enterprise Auth

- [ ] FEATURE: Integracion LDAP/AD
  - Prioridad: p2
  - Labels: feature, security, api
  - Criterios de aceptacion:
    - Bind seguro y mapeo de grupos.
    - Fallback local de emergencia.

- [ ] FEATURE: WebAuthn/passkeys
  - Prioridad: p2
  - Labels: feature, security, frontend
  - Criterios de aceptacion:
    - Alta/baja de credenciales.
    - Step-up para acciones sensibles.
    - 2FA por usuario.

---

## Milestones

- [x] v0.9 Resolver: EPIC-01 + base de EPIC-03.
- [x] v1.0 Policy: EPIC-02 + resto de EPIC-03 + privacidad base.
- [x] v1.1 Cluster: EPIC-05 (auth, outbox persistente), importador Pi-hole y API keys con scopes.
- [ ] v1.2 Enterprise: auth avanzada + consistencia dual-stack + ecosistema IaC/SDK.

Nota: el CHANGELOG va por 0.93.0 mientras que v0.9 y v1.0 ya estan cubiertos funcionalmente. Decidir si se salta a 1.0.0 en la proxima release.

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
