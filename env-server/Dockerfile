#
# Environmental Data Server (SMH)
#

# Start with a builder stage; later we'll copy only the required files to the
# final, very slim image.
FROM rust:1.87-slim as builder

# Use a a dummy main.rs file just to get a Docker layer with all the
# dependencies compiled.
WORKDIR /usr/src/env-server
COPY ./Cargo.* ./
RUN mkdir ./src && echo 'fn main() { println!("Dummy!"); }' > ./src/main.rs
RUN cargo build --release && rm src/*.rs

# Copy and build our own sources.
COPY ./src ./src
RUN touch -a -m ./src/main.rs
RUN cargo build --release --target-dir .

# At last create the final image.
FROM debian:12.11-slim
WORKDIR /env-server

RUN apt-get update
RUN apt-get install sqlite3

COPY --from=builder /usr/src/env-server/release/env-server ./env-server
CMD [ "./env-server" ]
