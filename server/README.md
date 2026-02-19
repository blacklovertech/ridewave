# RideWave 🌊 — High-Scale Ride-Hailing Platform

**RideWave** is a production-hardened, high-performance ride-hailing backend built with **Go (Golang)** and **Redis**. It is engineered to handle thousands of concurrent drivers and riders using the same architectural patterns as industry leaders like **Ola** and **Uber**.

---

## 🚀 "Ola/Uber Grade" Architecture & Optimizations

RideWave v2.5 introduces enterprise-grade performance and security optimizations.

### 📍 1. Redis-Exclusive Live Tracking (Zero DB Location IO)

To avoid database bottlenecks during high traffic, RideWave implements a **Redis-First** strategy for real-time data:

- **Live GPS Heartbeats**: Every driver update (`PUT /api/v1/driver/location`) writes exclusively to **Redis Geospatial Indexes (GEOADD)**.
- **Scaling**: Reduces PostgreSQL write IO by over **99%**, ensuring the main database stays fast even during peak hours.
- **Broadcast Efficiency**: Real-time broadcasts use high-speed Redis memory lookups, delivering driver positions with sub-millisecond latency.

### 🛡️ 2. Secure Booking via Route Caching

To prevent fare manipulation and improve mobile data efficiency:

- **Server-Side Validation**: Fares are calculated and routes are mapped server-side.
- **Route Cache**: The full route geometry (polyline) and fare are cached in Redis under a secure `RouteID`.
- **Booking Flow**: The frontend confirms a booking using only the `RouteID`, meaning high-stakes data like price and distance remains untouchable by client-side scripts.

### 📊 3. Centralized API Auditing & DB Optimization

We keep our primary database "clean" to ensure instant query responses:

- **Audit Logging**: Raw responses from third-party APIs (like Ola Maps) are stored in an `external_api_logs` table instead of the `rides` table.
- **Performance**: This removes bulky data strings like "Polylines" from transactional tables, reducing their size by ~80% and making SQL indexes much faster.

---

## 🛠️ External Service Integrations

RideWave leverages a suite of industry-standard APIs to provide a seamless experience:

| Service               | Category        | Features                                                                                          |
| :-------------------- | :-------------- | :------------------------------------------------------------------------------------------------ |
| **Ola Maps**          | Mapping & GIS   | `Directions` (Routing), `SnapToRoad` (Smoothing), `Autocomplete` (Search), `NearbySearch` (POIs). |
| **Twilio Verify**     | Auth & Identity | Secure `SMS OTP` for phone number verification (User/Driver login).                               |
| **Firebase (FCM)**    | Pub/Sub & Push  | `FCM Server Key` used for real-time ride dispatching and background alerts.                       |
| **SMTP (Nodemailer)** | Email Security  | `Email OTP` for high-security profile updates and admin verification.                             |

---

## 📡 Complete API Endpoints Reference

### 🏥 Health & Diagnostics

- `GET /health` — **Deep Diagnostics**: Returns system uptime, Go version, DB latency, and Redis connectivity stats.

### 👤 User Services (`/api/v1/user`)

| Method | Endpoint                    | Description                          |
| :----- | :-------------------------- | :----------------------------------- |
| `POST` | `/auth/login`               | Send SMS OTP (Twilio)                |
| `POST` | `/auth/verify`              | Verify OTP & Generate JWT            |
| `POST` | `/auth/logout`              | Invalidate session                   |
| `GET`  | `/me`                       | Get profile data                     |
| `PUT`  | `/profile`                  | Update name, email, etc.             |
| `PUT`  | `/notification-token`       | Update FCM device token              |
| `GET`  | `/vehicle-types`            | List available vehicle categories    |
| `GET`  | `/service-availability`     | Check if location is in service zone |
| `GET`  | `/places/autocomplete`      | Search locations (Ola Maps)          |
| `GET`  | `/places/nearby`            | Discover nearby pickup points        |
| `POST` | `/ride/estimate`            | Get fare + route geometry (Cached)   |
| `POST` | `/ride/create`              | Book ride using secure `RouteID`     |
| `POST` | `/ride/cancel`              | Terminate ride request               |
| `GET`  | `/ride/:id`                 | Detailed ride receipt                |
| `GET`  | `/ride/:id/driver-location` | Real-time driver tracking (Redis)    |
| `GET`  | `/rides`                    | Full trip history                    |
| `GET`  | `/payment/:rideId`          | Individual payment receipt           |
| `POST` | `/payment/verify-direct`    | Verify Cash/UPI transaction          |
| `POST` | `/rate-driver`              | Post-trip driver review              |
| `POST` | `/sos`                      | Immediate safety alert               |

### 🚗 Driver Services (`/api/v1/driver`)

| Method | Endpoint                  | Description                      |
| :----- | :------------------------ | :------------------------------- |
| `POST` | `/auth/login`             | Driver identity check            |
| `POST` | `/auth/verify`            | Authenticated driver session     |
| `POST` | `/auth/logout`            | Logout driver                    |
| `GET`  | `/me`                     | Detailed driver profile          |
| `PUT`  | `/status`                 | Update vehicle/doc details       |
| `PUT`  | `/toggle-online`          | Toggle availability              |
| `PUT`  | `/notification-token`     | Update FCM device token          |
| `GET`  | `/vehicle-types`          | List types for registration      |
| `PUT`  | `/location`               | **Ultra-Fast**: GPS (Redis-Only) |
| `GET`  | `/ride/:id/user-location` | Navigation coordinates           |
| `GET`  | `/incoming-ride`          | Fetch assigned requests          |
| `PUT`  | `/ride/status`            | Arrived, Started, Completed      |
| `GET`  | `/rides`                  | Driver trip history              |
| `GET`  | `/ride/:id`               | Specific ride manifest           |
| `POST` | `/rate-user`              | Post-trip user review            |
| `POST` | `/payment/confirm`        | Confirm payment received         |
| `GET`  | `/earnings`               | All-time balance dashboard       |
| `GET`  | `/earnings/daily`         | Today's revenue breakdown        |
| `GET`  | `/earnings/weekly`        | Weekly revenue breakdown         |
| `GET`  | `/list`                   | Search drivers by ID             |

### 🛡️ Admin Suite (`/api/v1/admin`)

| Method   | Endpoint             | Description                          |
| :------- | :------------------- | :----------------------------------- |
| `GET`    | `/dashboard`         | Platform Master KPIs                 |
| `POST`   | `/email-otp-request` | Admin email verification             |
| `PUT`    | `/email-otp-verify`  | Admin identity confirmation          |
| `GET`    | `/users`             | Global user directory                |
| `GET`    | `/user/:id`          | User deep-dive data                  |
| `PUT`    | `/user/:id/status`   | Ban/Suspend/Activate user            |
| `GET`    | `/drivers`           | Global driver directory              |
| `GET`    | `/driver/:id`        | Document & RC verification           |
| `PUT`    | `/driver/:id/status` | Approve registration/RC              |
| `GET`    | `/drivers/live`      | **Live Map**: Real-time traffic view |
| `GET`    | `/rides`             | Global ride monitor                  |
| `GET`    | `/ride/:id`          | Ride forensic audit                  |
| `GET`    | `/payments`          | Financial audit log                  |
| `GET`    | `/vehicle-types`     | Manage fleet categories              |
| `PUT`    | `/vehicle-type`      | Upsert pricing/details               |
| `DELETE` | `/vehicle-type/:id`  | Remove category                      |
| `GET`    | `/sos-alerts`        | Dispatch safety response             |
| `PUT`    | `/sos/:id/resolve`   | Close safety incident                |
| `GET`    | `/promo-codes`       | Marketing dashboard                  |
| `POST`   | `/promo-code`        | Create discount code                 |
| `PUT`    | `/promo-code/:id`    | Edit active promo                    |
| `DELETE` | `/promo-code/:id`    | Deactivate promotion                 |
| `GET`    | `/analytics/daily`   | Revenue & Growth reports             |

---

## 🛠️ Technical Setup

### Prerequisites

- **Go 1.21+**
- **PostgreSQL 15+**
- **Redis 7.0+**
- **Ola Maps / Twilio / FCM Keys**

### Developer Launch

1.  `go mod tidy`
2.  `go run main.go migrate` (Setup database)
3.  `go run main.go server` (Start backend)

---

**Building the Future of Urban Mobility** | _Optimized for scale, secured for trust._
