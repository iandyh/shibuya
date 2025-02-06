
from locust import HttpUser, task, events

class HelloWorldUser(HttpUser):
    host = "http://blazedemo.com"

    @task
    def hello_world(self):
        #self.client.get("/asdf")

        self.client.get("/")

# from locust_plugins.listeners import jmeter
# @events.init.add_listener
# def on_locust_init(environment, **kwargs):
#     jmeter.JmeterListener(env=environment, testplan="examplePlan",
#                           flush_size=1, results_filename="/shibuya-agent/test-result/1.csv")
