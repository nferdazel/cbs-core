# Multi-stage Dockerfile for Next.js Backoffice
FROM node:22-alpine AS base

FROM base AS builder
WORKDIR /app
RUN corepack enable && corepack prepare pnpm@11.25.0 --activate

COPY package.json pnpm-workspace.yaml pnpm-lock.yaml* turbo.json ./
COPY packages/ packages/
COPY apps/backoffice-web/ apps/backoffice-web/

RUN pnpm install --frozen-lockfile
RUN pnpm --filter backoffice-web build

FROM base AS runner
WORKDIR /app

ENV NODE_ENV=production
ENV PORT=3000

RUN addgroup --system --gid 1001 nodejs && adduser --system --uid 1001 nextjs

COPY --from=builder /app/apps/backoffice-web/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/apps/backoffice-web/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/apps/backoffice-web/.next/static ./apps/backoffice-web/.next/static

USER nextjs

EXPOSE 3000

CMD ["node", "apps/backoffice-web/server.js"]
