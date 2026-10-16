# NexaCampus SchoolMaker Orchestrator API

## Endpoints
- POST /api/v1/maker/auth/send-otp: Sends 6-digit OTP to creator phone.
- POST /api/v1/maker/auth/verify-otp: Verifies OTP and returns creator session token.
- GET /api/v1/schools/check-slug?slug=...: Verifies subdomain slug availability.
- POST /api/v1/schools: Creates a new school tenant.
- GET /api/v1/schools/:slug: Fetches school tenant profile and active modules.
- PATCH /api/v1/schools/:slug/modules: Updates module toggle states.
- POST /api/v1/maker/provision: End-to-end institutional build & Vercel provisioning.
