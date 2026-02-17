# ⚡ Free API — freeapi.kidpech.app

REST API ฟรีสำหรับเรียนรู้และทดสอบ — JWT Auth, CRUD, Pagination, Rate Limiting

**🌐 Live:** [https://freeapi.kidpech.app](https://freeapi.kidpech.app)  
**📖 Docs:** [https://freeapi.kidpech.app](https://freeapi.kidpech.app) (Interactive docs with playground)

---

## Features

- 🔐 **JWT Authentication** — Register, Login, Refresh Token
- 📝 **Full CRUD** — Create, Read, Update (PUT/PATCH), Delete, Bulk operations
- 📄 **Pagination** — Offset-based & Cursor-based with search
- 🛡️ **Rate Limiting** — 60 req/min per IP (Redis or in-memory)
- 👥 **Role-based Access** — User & Admin roles
- 📊 **Monitoring** — Prometheus metrics, Sentry error tracking
- ⚡ **Production-grade** — Structured logging (Zap), graceful shutdown

## Quick Start

```bash
# Health check
curl https://freeapi.kidpech.app/api/v1/health

# Register
curl -X POST https://freeapi.kidpech.app/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"you@example.com","password":"MySecret123","name":"Your Name"}'

# Login
curl -X POST https://freeapi.kidpech.app/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"you@example.com","password":"MySecret123"}'
```

## API Endpoints

| Method | Path                    | Auth   | Description          |
| ------ | ----------------------- | ------ | -------------------- |
| GET    | `/api/v1/health`        | Public | Health check         |
| GET    | `/api/v1/metrics`       | Public | Prometheus metrics   |
| POST   | `/api/v1/auth/register` | Public | Register new user    |
| POST   | `/api/v1/auth/login`    | Public | Login                |
| POST   | `/api/v1/auth/refresh`  | Public | Refresh token        |
| GET    | `/api/v1/users/me`      | Auth   | Get current user     |
| PUT    | `/api/v1/users/me`      | Auth   | Update current user  |
| GET    | `/api/v1/admin/users`   | Admin  | List all users       |
| POST   | `/api/v1/profiles`      | Auth   | Create profile       |
| POST   | `/api/v1/profiles/bulk` | Auth   | Bulk create profiles |
| GET    | `/api/v1/profiles`      | Auth   | List profiles        |
| GET    | `/api/v1/profiles/:id`  | Auth   | Get profile          |
| PUT    | `/api/v1/profiles/:id`  | Auth   | Update profile       |
| PATCH  | `/api/v1/profiles/:id`  | Auth   | Patch profile        |
| DELETE | `/api/v1/profiles/:id`  | Auth   | Delete profile       |
| DELETE | `/api/v1/profiles/bulk` | Auth   | Bulk delete          |

## Tech Stack

| Component  | Technology                |
| ---------- | ------------------------- |
| Language   | Go 1.23                   |
| Framework  | Gin                       |
| Database   | PostgreSQL (sqlx + pgx)   |
| Cache      | Redis (go-redis)          |
| Auth       | JWT (golang-jwt) + bcrypt |
| Logging    | Zap                       |
| Monitoring | Prometheus + Sentry       |
| Container  | Docker (distroless)       |
| Hosting    | Railway                   |

## Deploy to Railway

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/template)

1. Fork this repo
2. Connect to Railway
3. Add a PostgreSQL plugin
4. Set environment variables:
   ```
   PORT=8080
   APP_ENV=production
   DB_DRIVER=postgres
   DB_DSN=<railway-postgres-url>
   JWT_ACCESS_SECRET=<your-secret>
   JWT_REFRESH_SECRET=<your-secret>
   ALLOWED_HOSTS=freeapi.kidpech.app
   CORS_ORIGINS=https://freeapi.kidpech.app
   ```
5. Deploy!

### Environment Variables

| Variable             | Default                       | Description                     |
| -------------------- | ----------------------------- | ------------------------------- |
| `PORT`               | `8080`                        | Server port (Railway sets this) |
| `APP_ENV`            | `development`                 | `development` or `production`   |
| `DB_DRIVER`          | `postgres`                    | `postgres` or `mysql`           |
| `DB_DSN`             | —                             | Database connection string      |
| `REDIS_ADDR`         | —                             | Redis address (optional)        |
| `JWT_ACCESS_SECRET`  | —                             | JWT signing secret              |
| `JWT_REFRESH_SECRET` | —                             | Refresh token secret            |
| `ALLOWED_HOSTS`      | `freeapi.kidpech.app`         | Allowed hostnames               |
| `CORS_ORIGINS`       | `https://freeapi.kidpech.app` | CORS origins                    |
| `RATE_LIMIT_ENABLED` | `true`                        | Enable rate limiting            |
| `RATE_LIMIT_PER_MIN` | `60`                          | Requests per minute             |
| `ALLOW_REGISTRATION` | `true`                        | Allow user registration         |

## Local Development

```bash
# Run with Docker Compose
docker-compose up -d

# Or run directly
cp .env.example .env  # edit values
make run
```

## License

See [LICENSE](LICENSE) file.
