FROM debian:bookworm-slim AS install
WORKDIR /app
COPY . .
RUN sh -c 'pip install pipenv && pipenv install --deploy --system'

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=install /app /app
CMD ["python", "main.py"]
