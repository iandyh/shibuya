FROM locustio/locust

RUN pip install locust-plugins hyper

USER root
RUN mkdir /test-result /test-data

ADD build/shibuya-agent /usr/local/bin/shibuya-agent
ADD locustfile.py locustfile.py

ENTRYPOINT []
CMD ["shibuya-agent"]


