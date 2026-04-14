# Webcam API

> REST microservice for ingesting webcam images, storing them in S3, and queuing downstream processing events via SQS.

Part of the **Weather Archive** platform -- see also [video-service](https://github.com/direncanS/video-service-main) and [user-api](https://github.com/direncanS/user-api-main).

Built with **Go 1.19** | **AWS Lambda** | **PostgreSQL** | **S3** | **SQS** | **JWT**

---

## Overview

The Webcam API is the ingestion layer of the Weather Archive system. IoT devices and webcam clients authenticate via JWT, then submit base64-encoded images along with topic metadata. The service decodes and stores images in S3, indexes metadata in PostgreSQL, and publishes processing events to an SQS queue for downstream consumers.

## Features

- **JWT Authentication** -- clients register with a name and receive a 24-hour HS256 token
- **Image Ingestion** -- accepts base64-encoded JPEG payloads up to 1 MB
- **S3 Storage** -- uploads decoded images with unique object keys
- **Async Processing** -- publishes SQS messages with a configurable delay for downstream video compilation
- **PostgreSQL Indexing** -- stores image metadata (topic, timestamps, JSONB metadata) for querying
- **Dual Deployment** -- runs locally on port 8080 or as an AWS Lambda function (auto-detected)

## Architecture

```
Client (IoT / Webcam)
        │
        ▼
   Webcam API (Lambda / HTTP)
   ├── JWT auth middleware
   ├── Base64 decode + S3 upload
   ├── PostgreSQL metadata insert
   └── SQS event publish
        │                │
        ▼                ▼
   S3 Bucket       SQS Queue
                        │
                        ▼
                  Video Service
```

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/login` | None | Register a webcam and receive a JWT token |
| POST | `/` | JWT | Submit webcam image with topic and metadata |

### POST /login

```json
{ "name": "rooftop-cam-01" }
```

Returns a 24-hour JWT token.

### POST /

```json
{
  "image": "<base64-encoded JPEG>",
  "topic": "vienna-skyline",
  "metadata": { "location": "rooftop", "device_id": "cam-01" }
}
```

Returns `201 CREATED` on success.

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.19 |
| Runtime | AWS Lambda (ARM64) / standalone HTTP server |
| Router | Gorilla Mux with Lambda proxy adapter |
| Database | PostgreSQL (pgx v5) |
| Object Storage | AWS S3 (SDK v2) |
| Message Queue | AWS SQS (SDK v2) |
| Auth | golang-jwt v5 (HS256) |
| Logging | Uber Zap |

## Database Schema

```sql
CREATE TABLE webcam_data (
    id                SERIAL PRIMARY KEY,
    image_object_key  VARCHAR(255) NOT NULL,
    topic             VARCHAR(255) NOT NULL,
    metadata          JSONB NOT NULL,
    created_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    used              BOOLEAN DEFAULT FALSE
);
```

## Getting Started

### Prerequisites

- Go 1.19+
- PostgreSQL instance
- AWS credentials with S3 and SQS access

### Environment Variables

```bash
host=localhost
password=<postgres-password>
SECRET_KEY=<jwt-signing-secret>
S3_BUCKET=<s3-bucket-name>
```

### Run Locally

```bash
make build && make run
# Server starts on :8080
```

### Deploy to AWS Lambda

```bash
make buildAWS   # ARM64 binary
make zip         # Creates bootstrap.zip for Lambda upload
```

## Related Services

| Service | Purpose |
|---------|---------|
| [video-service](https://github.com/direncanS/video-service-main) | Compiles stored webcam images into video sequences per topic |
| [user-api](https://github.com/direncanS/user-api-main) | Client-facing API for querying webcam data and video metadata |
