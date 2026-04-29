FROM debian:bookworm-slim AS install
WORKDIR /app
COPY . .
RUN sh -c 'pip install -r requirements.txt'

FROM debian:bookworm-slim
RUN useradd -r -u 10001 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=install --chown=appuser:appuser /app /app
USER appuser
CMD ["gunicorn", "app:app"]
