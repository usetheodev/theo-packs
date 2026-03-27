from celery import Celery

app = Celery("worker")

@app.task
def process():
    return "done"
