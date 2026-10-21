# سرویس مدیریت وظایف (Task Manager)

این پروژه یک سرویس کوچک بک‌اند برای مدیریت وظایف است. کلاینت از طریق API مبتنی بر REST با سرویس ارتباط می‌گیرد. **PostgreSQL تنها پایگاه داده و منبع اصلی ذخیره‌سازی اطلاعات وظایف است.** Redis فقط برای کش‌کردن پاسخ فهرست وظایف به‌کار می‌رود؛ اگر Redis در دسترس نباشد یا تنظیم نشده باشد، عملیات همچنان مستقیماً با PostgreSQL انجام می‌شوند.

## معماری

```mermaid
flowchart LR
    Client[کلاینت] --> API[Gin و API مبتنی بر REST]
    API --> Service[سرویس و اعتبارسنجی]
    Service --> Cache[کش اختیاری Redis برای فهرست]
    Cache --> Repository[مخزن PostgreSQL]
    Repository --> DB[(PostgreSQL)]
    API --> Metrics[Prometheus: /metrics]
    API --> Tracing[OpenTelemetry: خروجی در stdout]
```

مسیر معمول یک درخواست:

```text
درخواست HTTP → هندلر Gin → سرویس وظایف → مخزن PostgreSQL → پایگاه داده PostgreSQL
```

در `GET /tasks`، اگر کش Redis فعال باشد ابتدا نتیجه از کش خوانده می‌شود. در صورت cache miss یا خطای Redis، داده از PostgreSQL گرفته می‌شود. پس از ایجاد، ویرایش یا حذف موفق، کلیدهای فهرست در Redis پاک می‌شوند. دادهٔ اصلی هیچ‌وقت فقط در Redis نگهداری نمی‌شود.

## ساختار کد

- `cmd/api/main.go`: خواندن تنظیمات، اتصال به PostgreSQL، اجرای migration، تنظیم tracing و راه‌اندازی وب‌سرور. Redis اختیاری است.
- `internal/task/http.go`: مسیرهای Gin، خواندن ورودی JSON و ساخت پاسخ‌های HTTP.
- `internal/task/service.go`: قواعد و اعتبارسنجی وظایف؛ مستقل از Gin و SQL.
- `internal/task/repository.go`: اینترفیس مخزن و پیاده‌سازی کوئری‌های پارامتری PostgreSQL.
- `internal/task/cache.go` و `internal/task/redis_cache.go`: کش اختیاری فهرست وظایف و اتصال Redis.
- `internal/task/model.go`: مدل وظیفه، وضعیت‌ها و قواعد اعتبارسنجی.
- `migrations/001_create_tasks.sql`: ساخت جدول `tasks` در PostgreSQL.
- `openapi.yaml`: مشخصات OpenAPI سرویس.

## مدل وظیفه و قواعد API

هر وظیفه شامل این فیلدهاست:

| فیلد | توضیح |
| --- | --- |
| `id` | شناسهٔ UUID که PostgreSQL می‌سازد |
| `title` | عنوان الزامی، پس از حذف فاصله‌های ابتدا و انتها؛ حداکثر ۲۰۰ نویسه |
| `description` | توضیح اختیاری؛ حداکثر ۵۰۰۰ نویسه |
| `status` | یکی از `pending`، `in_progress` یا `completed`؛ مقدار پیش‌فرض `pending` |
| `assignee` | مسئول اختیاری؛ حداکثر ۲۰۰ نویسه |
| `created_at` | زمان ایجاد |
| `updated_at` | زمان آخرین ویرایش |

`PUT` همهٔ فیلدهای قابل‌ویرایش را جایگزین می‌کند. اگر `description` یا `assignee` ارسال نشوند، خالی می‌شوند؛ اگر `status` ارسال نشود، مقدار آن `pending` خواهد بود. فهرست بر اساس زمان ایجاد، از جدید به قدیم مرتب می‌شود.

### مسیرها

| روش | مسیر | نتیجهٔ موفق |
| --- | --- | --- |
| `POST` | `/tasks` | `201 Created` به‌همراه وظیفه و هدر `Location` |
| `GET` | `/tasks` | `200 OK` به‌همراه فهرست صفحه‌بندی‌شده |
| `GET` | `/tasks/{id}` | `200 OK` به‌همراه یک وظیفه |
| `PUT` | `/tasks/{id}` | `200 OK` به‌همراه وظیفهٔ ویرایش‌شده |
| `DELETE` | `/tasks/{id}` | `204 No Content` |

فهرست وظایف امکان فیلتر دقیق با `status` و `assignee` را دارد. صفحه‌بندی با `page` و `page_size` انجام می‌شود: شمارهٔ صفحه از ۱ شروع می‌شود، مقدار پیش‌فرض صفحه ۱ و اندازهٔ صفحه ۲۰ است و حداکثر اندازهٔ صفحه ۱۰۰ است.

خطای ورودی یا شناسهٔ نامعتبر پاسخ `400` با قالب `{"error":"..."}` می‌دهد. برای UUID معتبر که وظیفه‌ای با آن وجود ندارد پاسخ `404` برمی‌گردد. خطای داخلی با پاسخ عمومی `500` داده می‌شود و جزئیات پایگاه داده به کلاینت فرستاده نمی‌شود.

نمونهٔ پاسخ فهرست:

```json
{
  "items": [
    {
      "id": "e1f3d6b6-41f2-4910-9352-bd91856698a4",
      "title": "انتشار نسخهٔ اول",
      "description": "تکمیل API",
      "status": "pending",
      "assignee": "مینا",
      "created_at": "2026-10-19T10:00:00Z",
      "updated_at": "2026-10-19T10:00:00Z"
    }
  ],
  "page": 1,
  "page_size": 20,
  "total": 1,
  "total_pages": 1
}
```

## پیش‌نیاز و اجرا

برای اجرای محلی به Docker و Docker Compose نیاز دارید:

```bash
docker compose up --build
```

سرویس API روی `http://localhost:8080` در دسترس است. Compose سه سرویس را اجرا می‌کند:

- `api`: سرویس Go
- `postgres`: پایگاه داده PostgreSQL و فضای ذخیره‌سازی پایدار آن
- `redis`: کش فهرست وظایف

برنامه هنگام شروع برای اتصال به PostgreSQL تا ۳۰ ثانیه تلاش می‌کند و migration اولیه را اجرا می‌کند. رمز PostgreSQL در Compose فقط برای توسعهٔ محلی است.

برای اجرای فقط PostgreSQL، مثلاً جهت تست یکپارچه‌سازی:

```bash
docker compose up -d postgres
```

### تنظیمات محیطی

| متغیر | پیش‌فرض | کاربرد |
| --- | --- | --- |
| `DATABASE_URL` | الزامی | آدرس اتصال PostgreSQL |
| `HTTP_ADDR` | `:8080` | نشانی و درگاه HTTP |
| `GIN_MODE` | `release` | حالت Gin |
| `REDIS_ADDR` | غیرفعال | نشانی Redis؛ مثلاً `localhost:6379` |
| `REDIS_PASSWORD` | خالی | رمز Redis |
| `REDIS_DB` | `0` | شمارهٔ پایگاه دادهٔ Redis |

اگر `REDIS_ADDR` تنظیم نشده باشد، سرویس بدون Redis و با PostgreSQL کار می‌کند.

## مستندات API و نمونهٔ استفاده

فایل OpenAPI در ریشهٔ پروژه قرار دارد و از این مسیرها خوانده می‌شود:

- مشخصات OpenAPI: `http://localhost:8080/openapi.yaml`
- مسیر مستندات خوانا: `http://localhost:8080/swagger`

نمونهٔ درخواست‌ها:

```bash
# ساخت وظیفه
curl -i -X POST http://localhost:8080/tasks \
  -H 'Content-Type: application/json' \
  -d '{"title":"انتشار نسخهٔ اول","description":"تکمیل API","assignee":"مینا"}'

# فهرست، فیلتر و صفحه‌بندی
curl 'http://localhost:8080/tasks?status=pending&assignee=%D9%85%DB%8C%D9%86%D8%A7&page=1&page_size=10'

# دریافت با UUID پاسخ POST
curl http://localhost:8080/tasks/<uuid>

# ویرایش کامل فیلدهای قابل‌ویرایش
curl -X PUT http://localhost:8080/tasks/<uuid> \
  -H 'Content-Type: application/json' \
  -d '{"title":"انتشار نسخهٔ اول","status":"in_progress","description":"بازبینی نهایی","assignee":"مینا"}'

# حذف
curl -i -X DELETE http://localhost:8080/tasks/<uuid>
```

## تست‌ها و پوشش کد

اجرای تست‌های واحد و تست‌های مخزن با SQL mock:

```bash
go test ./...
go test ./internal/task -cover
```

در آخرین اجرای ثبت‌شده، پوشش پکیج `internal/task` برابر **۷۸٫۵٪** بود. تست یکپارچه‌سازی PostgreSQL به یک پایگاه دادهٔ تست اختصاصی نیاز دارد. جدول‌ها در یک schema موقت ساخته و پس از تست پاک می‌شوند:

```bash
TEST_DATABASE_URL='postgres://tasks:tasks@localhost:5432/tasks?sslmode=disable' \
  go test ./internal/task -run TestPostgresIntegrationCRUD -count=1
```

این تست migration و عملیات CRUD، فیلتر و صفحه‌بندی را روی PostgreSQL واقعی اجرا می‌کند و مسیر HTTP و پاک‌شدن کش پس از تغییر داده را هم بررسی می‌کند. تست‌های معمولی با fake مخزن و `go-sqlmock` به دیتابیس واقعی نیاز ندارند.

اجرای benchmark و ساخت پروفایل CPU و حافظه:

```bash
go test ./internal/task -run '^$' -bench BenchmarkListTasksCacheHit \
  -benchmem -cpuprofile cpu.out -memprofile mem.out
go tool pprof -top cpu.out
go tool pprof -top -alloc_space mem.out
```

در یک اجرای محلی با Go 1.26.3 و پردازندهٔ Intel Core i7-6700HQ، benchmark مسیر cache-hit حدود `9.2 µs/op`، `960 B/op` و `17 allocs/op` ثبت کرد. در پروفایل CPU، SHA-256 و decode کردن JSON از هزینه‌های قابل‌مشاهده بودند؛ در پروفایل تخصیص حافظه، decode نتیجهٔ کش‌شده و ساخت کلید کش سهم بیشتری داشتند. این benchmark از کش درون‌حافظه‌ای استفاده می‌کند و زمان شبکهٔ Redis یا PostgreSQL را اندازه نمی‌گیرد؛ بنابراین اعداد آن فقط یک baseline محلی هستند.

## پایش و مشاهده‌پذیری

مسیر `http://localhost:8080/metrics` معیارهای Prometheus را ارائه می‌کند:

- `requests_total`: تعداد درخواست‌ها بر اساس روش، مسیر و وضعیت HTTP
- `request_latency_histogram`: زمان پاسخ‌گویی درخواست‌ها
- `tasks_count`: تعداد فعلی وظایف ذخیره‌شده در PostgreSQL

برای هر درخواست HTTP یک span با OpenTelemetry ساخته می‌شود. span روش، مسیر و کد وضعیت را ثبت می‌کند و exporter محلی آن را به‌شکل خوانا در stdout می‌نویسد. برای مشاهدهٔ خروجی از `docker compose logs api` استفاده کنید. در محیط عملیاتی می‌توان exporter را به OTLP و یک collector متصل کرد.

## تصمیم‌های طراحی و مصالحه‌ها

- سرویس یک واحد قابل‌اجراست و مسیر HTTP، قواعد وظایف و ذخیره‌سازی در لایه‌های جدا قرار دارند. gRPC اضافه نشده چون کلاینت این پروژه REST می‌خواهد.
- PostgreSQL منبع اصلی داده است. کوئری‌ها پارامتری هستند تا ورودی کاربر با متن SQL ترکیب نشود.
- Redis فقط برای فهرست‌هاست. پس از هر نوشتن موفق، همهٔ کلیدهای فهرست پاک می‌شوند؛ این روش ساده است اما با بزرگ‌شدن تعداد کلیدها می‌توان invalidation دقیق‌تری طراحی کرد. TTL پنج‌دقیقه‌ای هم مدت باقی‌ماندن دادهٔ قدیمی را محدود می‌کند.
- صفحه‌بندی حداکثر ۱۰۰ مورد در هر درخواست برمی‌گرداند. شمارش کل و دریافت صفحه دو کوئری جدا هستند، پس در حضور نوشتن هم‌زمان snapshot یکسانی تضمین نمی‌شود.
- migration اولیه idempotent است و جدول را در صورت نبودن می‌سازد. برای تغییرات متعدد schema بهتر است از ابزار migration نسخه‌بندی‌شده استفاده شود.
- exporter فعلی tracing برای اجرای محلی و مصاحبه ساده است؛ برای محیط production بهتر است traceها به سامانهٔ مانیتورینگ ارسال شوند.
