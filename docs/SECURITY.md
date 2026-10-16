# Security Architecture

- **Token Format**: 256-bit cryptographic creator session tokens.
- **Slug Guard**: Reserved keywords (admin, api, platform, www, maker) are blocked.
- **Rate Limiting**: In-memory rate limiting on OTP request endpoints.
