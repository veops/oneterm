FROM node:16.20.0-alpine AS builder
WORKDIR /oneterm-ui
COPY . .
RUN yarn config set registry https://registry.npmmirror.com/
RUN yarn install
RUN yarn build

FROM nginx:alpine
COPY --from=0 /oneterm-ui/dist /etc/nginx/html