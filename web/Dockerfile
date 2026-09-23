FROM node:22-alpine AS build

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

# Baked in at build time: a Vite bundle is static files, so there is no
# runtime to read an environment variable.
ARG API_BASE_URL=""
ARG STRIPE_PUBLISHABLE_KEY=""
RUN npm run build

FROM nginxinc/nginx-unprivileged:1.27-alpine

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 8080
