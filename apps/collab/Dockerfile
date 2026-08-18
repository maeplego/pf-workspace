FROM node:22-alpine
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm install --omit=dev
COPY src ./src
EXPOSE 8097
ENV COLLAB_PORT=8097
CMD ["node", "src/server.mjs"]
