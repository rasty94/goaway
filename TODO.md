# GoAway - Hoja de Ruta y TODO

Ultima actualizacion: 2026-11-25

Este archivo solo contiene trabajo pendiente. Lo completado esta en [DONE.md](DONE.md).

Tablero recomendado (GitHub Projects): Inbox, Ready, In Progress, Review, Blocked, Done.

Labels base: epic, feature, enhancement, security, dns, dhcp, ha, frontend, api, observability, migration, breaking-change, good-first-issue, p0, p1, p2.

---

## Epicas abiertas

| Epic | Objetivo | Prioridad | Estado |
| --- | --- | ---: | --- |
| EPIC-11 Paridad Final & v1.0.0 | Release v1.0.0 | P1 | En curso (falta el tag) |

Solo queda el corte de la release.

---

## P0 - Deuda que bloquea lo demas

_(sin tareas)_

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

- [ ] RELEASE: Corte de v1.0.0
  - Prioridad: p1
  - Labels: migration
  - Contexto: guia de migracion, CHANGELOG, versiones a 1.0.0 y URLs del fork ya hechas (ver DONE.md).
  - Criterios de aceptacion:
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
