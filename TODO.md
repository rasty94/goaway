# GoAway - Hoja de Ruta y TODO

Ultima actualizacion: 2026-11-25

Este archivo solo contiene trabajo pendiente. Lo completado esta en [DONE.md](DONE.md).

Tablero recomendado (GitHub Projects): Inbox, Ready, In Progress, Review, Blocked, Done.

Labels base: epic, feature, enhancement, security, dns, dhcp, ha, frontend, api, observability, migration, breaking-change, good-first-issue, p0, p1, p2.

---

## Epicas abiertas

| Epic | Objetivo | Prioridad | Estado |
| --- | --- | ---: | --- |
| EPIC-11 Paridad Final & v1.0.0 | Release v1.0.0 | P1 | Hecho (v1.0.0 publicada); solo queda TS 7, bloqueado |

v1.0.0 publicada el 2026-11-25.

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
    - Actualizar cuando `typescript-eslint` publique soporte para TS 7; `pnpm -C client exec eslint`, `tsc -b`, build y Vitest en verde.


---

## P2 - Mejoras menores

- [ ] FRONTEND: UI para dar de alta 2FA (TOTP) y gestionar passkeys (WebAuthn)
  - Prioridad: p2
  - Labels: frontend, security
  - Contexto: el backend expone TOTP y WebAuthn (`/api/auth/webauthn/register/*`, `/api/auth/webauthn/credentials`), pero el cliente solo pide el codigo TOTP en el login; no hay pantalla para activar 2FA, registrar passkeys ni iniciar sesion con ellas.
  - Criterios de aceptacion:
    - Seccion en ajustes para activar/desactivar TOTP (QR + codigo de verificacion) y listar, registrar y borrar passkeys.
    - Boton de login con passkey cuando `auth/webauthn/status` lo indique.
    - Tests de componente con el mock de `navigator.credentials`.

---

## Milestones

- [x] v0.9 Resolver: EPIC-01 + base de EPIC-03.
- [x] v1.0 Policy: EPIC-02 + resto de EPIC-03 + privacidad base.
- [x] v1.1 Cluster: EPIC-05 (auth, outbox persistente, resync logico, reloj logico, nombres estables, mTLS), importador Pi-hole y API keys con scopes.
- [x] v1.0.0 publicada (2026-11-25): binarios en GitHub Releases e imagen multi-arch en `ghcr.io/rasty94/goaway:1.0.0`.
- [x] v1.2 Enterprise: auth avanzada (OIDC, LDAP/AD, TOTP 2FA, WebAuthn/passkeys) + consistencia dual-stack + ecosistema IaC (Helm chart + Terraform provider) + paridad resolver (DoQ, parallel_best, EDE, qtype, Redis, exportador JSON/CSV/Postgres).


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
