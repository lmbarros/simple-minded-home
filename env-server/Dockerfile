#
# Environmental Data Server (SMH)
#

# Builder stage.
FROM rust:1.87-slim AS builder

WORKDIR /usr/src/env-server
COPY Cargo.* ./
COPY src ./src
RUN cargo build --release --target-dir .

# Final image.
FROM debian:12.11-slim

WORKDIR /env-server

RUN apt-get update
RUN apt-get install -y sqlite3

COPY --from=builder /usr/src/env-server/release/env-server ./
COPY ./frontend ./frontend
COPY ./Rocket.toml ./

CMD [ "./env-server" ]
