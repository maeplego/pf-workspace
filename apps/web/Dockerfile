FROM node:22-alpine AS deps
WORKDIR /app
COPY apps/web/package.json ./
RUN npm install

FROM node:22-alpine AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY apps/web/ ./
ENV NEXT_TELEMETRY_DISABLED=1
ARG NEXT_PUBLIC_COLLAB_WS_URL=ws://localhost:8097
ENV NEXT_PUBLIC_COLLAB_WS_URL=$NEXT_PUBLIC_COLLAB_WS_URL
ARG NEXT_PUBLIC_CHAT_WS_URL=ws://localhost:8096/chat/ws
ENV NEXT_PUBLIC_CHAT_WS_URL=$NEXT_PUBLIC_CHAT_WS_URL
RUN npm run build

FROM node:22-alpine AS run
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3006
ENV HOSTNAME=0.0.0.0
COPY --from=build /app/public ./public
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static
EXPOSE 3006
CMD ["node", "server.js"]
