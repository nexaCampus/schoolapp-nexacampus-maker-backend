# System Architecture

The NexaCampus platform orchestrates autonomous multi-tenant schools:
1. **Next.js Edge Middleware**: Subdomain resolution (<slug>.nexacampus.tech).
2. **Go Backend Service**: Deployed on Render, handling tenancy, OTP, and provisioning.
3. **Vercel REST API**: Registers production domain aliases dynamically.
4. **Supabase / PostgreSQL**: Scalable master storage for institutions.
