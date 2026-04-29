FROM debian:bookworm-slim AS install
WORKDIR /app
COPY . .
RUN sh -c 'pip install -r requirements.txt'

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=install /app /app
CMD ["gunicorn", "app:app"]
