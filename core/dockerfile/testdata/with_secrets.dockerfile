FROM debian:bookworm-slim AS install
WORKDIR /app
COPY . .
RUN --mount=type=secret,id=API_KEY \
    --mount=type=secret,id=DATABASE_URL \
    sh -c 'pip install -r requirements.txt'

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=install /app /app
CMD ["/bin/bash", "-c", "python app.py"]
