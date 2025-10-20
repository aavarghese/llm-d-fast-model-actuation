# Copyright 2025 IBM.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import subprocess
import time
import re
from kubernetes import client, config, watch

NAMESPACE = "default"
YAML_FILE = "server_request.yaml"  # Path to server-requesting yaml
REQUESTER_LABEL = "app=dp-example" 
SERVER_SUFFIX = "-server"   

def apply_yaml(yaml_file):
    print(f"Applying {yaml_file}...")
    subprocess.run(["kubectl", "apply", "-f", yaml_file], check=True)

def delete_yaml(yaml_file):
    print(f"Cleaning up resources from {yaml_file}...")
    subprocess.run(["kubectl", "delete", "-f", yaml_file, "--ignore-not-found=true"], check=False)

def get_pods_with_label(api, label_selector):
    pods = api.list_namespaced_pod(namespace=NAMESPACE, label_selector=label_selector).items
    return pods

def wait_for_ready_pod(api, pod_name, timeout=600):
    w = watch.Watch()
    start = time.time()
    for event in w.stream(api.list_namespaced_pod, namespace=NAMESPACE, timeout_seconds=timeout):
        pod = event["object"]
        if pod.metadata.name == pod_name:
            for cond in pod.status.conditions or []:
                if cond.type == "Ready" and cond.status == "True":
                    w.stop()
                    end = time.time()
                    print(f"✅ Pod {pod_name} is Ready after {end - start:.2f} seconds")
                    return end
    raise TimeoutError(f"Pod {pod_name} not ready within {timeout}s")

def main():
    config.load_kube_config()
    v1 = client.CoreV1Api()

    # Start with clean state
    delete_yaml(YAML_FILE)

    print("\n=== Milestone 1 Benchmark: DPC Startup Latency ===")
    print("Goal: Measure time from ReplicaSet apply -> server-running pod ready\n")

    start_time = time.time()
    apply_yaml(YAML_FILE)

    print("Waiting for server-requesting pod to appear...")
    requester_pod = None
    for _ in range(60):
        pods = get_pods_with_label(v1, REQUESTER_LABEL)
        if pods:
            requester_pod = pods[0]
            break
        time.sleep(2)

    if not requester_pod:
        print("❌ No requester pod appeared within 120s.")
        return

    requester_name = requester_pod.metadata.name

    print("Waiting for server-providing pod to become ready...")
    ready_time = wait_for_ready_pod(v1, requester_name)

    total_time = ready_time - start_time
    print(f"\n🚀 Benchmark result: {total_time:.2f} seconds from creation → ready")

    # Optional: cleanup
    delete_yaml(YAML_FILE)

if __name__ == "__main__":
    main()
