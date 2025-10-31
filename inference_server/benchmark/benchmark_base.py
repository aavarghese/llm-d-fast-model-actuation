# Copyright 2025 The llm-d Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# 	http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Standard imports
from random import randint

# from time import perf_counter
from typing import Any, Dict, List, Optional

# Local imports
from utils import BaseLogger, parse_request_args, replace_repo_variable


class DualPodsBenchmark:
    """Benchmark class for dual-pod inference server readineness."""

    def __init__(
        self,
        op_mode: str = "kind",
        simulation_delays: Optional[Dict[str, float]] = None,
        log_output_file: str = "metrics.log",
    ):
        """
        Initialize the benchmark class.

        :param op_mode: The operational mode for the benchmark (one of remote, kind, or
                        simulated)
        :param simulation_delays: Customized delays in secs for the simulated mode
                                  depending on the scenario
        """
        logger = BaseLogger(log_output_file, self.__class__.__name__)
        self.logger = logger.get_custom_logger()
        self.logger.info("Logger Type: %s" % (self.logger.name))
        self.op_mode = op_mode
        if op_mode == "kind":  # Default
            self.logger.info("Operating with kind cluster.")
            # Set context with a kind cluster.
        elif op_mode == "remote":
            self.logger.info("Operating with remote cluster.")
            # Load config for the remote cluster.
        elif op_mode == "simulated":
            self.logger.info("Operating in simulated mode.")
            # Load simulation parameters for the particular scenario.
        else:
            raise ValueError("Mode must be one of [kind, remote, simulated]")

        self.parsed_inputs = self.parse_inputs()
        input_str = self.describe_inputs()
        self.logger.info(input_str)
        # self.logger.info(f"Parsed Inputs: {self.parsed_inputs}")
        self.results: List[Dict[str, Any]] = []

    def describe_inputs(self):
        """Get pretty print version of the user inputs"""
        pretty_print_str = "Namespace: {} \n".format(self.parsed_inputs[0])
        pretty_print_str += "Request YAML File: {}\n".format(self.parsed_inputs[1])
        pretty_print_str += "Requester Pod Label: {} \n".format(self.parsed_inputs[2])
        pretty_print_str += "Requester Pod Image: {}".format(self.parsed_inputs[3])
        return pretty_print_str

    def parse_inputs(self) -> tuple:
        """Parse user inputs from the CLI."""
        all_args = parse_request_args()
        ns = all_args.namespace
        yaml_template = all_args.yaml
        requester_pod_label = all_args.label
        requester_img = all_args.image
        requester_img_tag = all_args.tag

        # Generate the request YAML from template and image details.
        request_yaml_file = replace_repo_variable(
            requester_img, requester_img_tag, yaml_template
        )

        return ns, request_yaml_file, requester_pod_label, requester_img_tag
        # return {
        #    "namespace": ns, "yaml": request_yaml_file,
        #    "label": requester_pod_label, "tag": requester_img_tag
        #    }

    def configure_scenario(self, scenario: str, **kwargs) -> Dict[str, Any]:
        """
        Configure benchmark settings based on the given scenario.

        :param scenario: Scenario name (e.g., "Fast Replica Scale Up")
        :param kwargs: Scenario-specific params such as number of GPUs, variants, etc.
        """
        config = {"scenario": scenario, "params": kwargs}

        if scenario == "Introducing New Variant":
            config["description"] = "Deploying new model variant"
            config["num_model_variants"] = kwargs.get("num_model_variants", 1)
        elif scenario == "Fast Replica Scale Up":
            config["description"] = "Scale up replicas"
            config["replicas"] = kwargs.get("replicas", 2)
        else:  # Deploy the given yaml and report.
            config["description"] = "Generic benchmark"
            config["replicas"] = kwargs.get("replicas", 1)

        return config

    def run_benchmark(
        self, scenario: str, iterations: int = 1, timeout: int = 600, **scenario_kwargs
    ) -> List[Dict[str, Any]]:
        """
        Run the benchmark for a given scenario.

        :param scenario: The scenario name.
        :param iterations: Number of iterations for run.
        :param timeout: Timeout for each run in seconds.
        :param scenario_kwargs: Parameters for configuring the scenario.
        :return: List of result dictionaries.
        """
        # config = self.configure_scenario(scenario, **scenario_kwargs)
        ns, yaml_file, pod_label, image = self.parsed_inputs

        self.results = []
        for i in range(iterations):
            self.logger.info(f"Running iteration {i+1} for scenario '{scenario}'")

            # start_time = perf_counter()
            try:
                self.logger.info(f"Applying YAML: {yaml_file}.")
                # TODO: Implement application for kind v remote v simulation.
                # apply_yaml_file(yaml_file)

                # Check for pod readiness.
                # podname = "my-request"
                # rq_ready, provider_ready, provider_mode = wait_for_dual_pods_ready(ns,
                #    podname, timeout)
                # TODO: Handle readiness check for M1 vs M2 vs M3 pods.
                # total_time = ready_time
                total_time = randint(1, 400)

                # Compile the result.
                result = {
                    "iteration": i + 1,
                    "scenario": scenario,
                    "rq_time": total_time,
                    # "prv_time": total_time,
                    # "availability_mode": provider_mode,
                    "availability_mode": "cold",
                    "success": True,
                }
            except Exception as e:
                self.logger.error(f"Iteration {i+1} failed with error: {e}")
                result = {
                    "iteration": i + 1,
                    "scenario": scenario,
                    "rq_time": None,
                    # "prv_time": total_time,
                    "availability_mode": "No Server Providing Pod Available",
                    "success": True,
                    "error": e.__str__(),
                }
            finally:
                self.logger.info(f"Finally deleting YAML file: {yaml_file}")
                # TODO: Implement deletion for kind v remote v simulation.
                # delete_yaml(yaml_file)

            self.results.append(result)

        return self.results

    def get_results(self) -> Dict[str, Any]:
        """
        Aggregate and return the benchmark results.

        :return: Dict with summary of stats (e.g., average, min, max, etc)
        """
        if not self.results:
            return {}

        success_runs = [run for run in self.results if run["success"]]
        rq_times = [
            run["rq_time"] for run in success_runs if run["rq_time"] is not None
        ]
        # TODO: Uncomment once we have run times for the provider pods.
        # prv_times = [run["prv_time"] for run in success_runs if run["prv_time"] is not None]

        summary = {
            "Total Runs": len(self.results),
            "Successful Runs": len(success_runs),
            "Failed Runs": len(self.results) - len(success_runs),
            "Average Requester Time": (
                sum(rq_times) / len(rq_times) if rq_times else None
            ),
            "Min Requester Time": min(rq_times) if rq_times else None,
            "Max Requester Time": max(rq_times) if rq_times else None,
            # "Average Provider Time": sum(prv_times) / len(prv_times) if prv_times else None,
            # "Min Provider Time": min(prv_times) if prv_times else None,
            # "Max Provider Time": max(prv_times) if prv_times else None,
            "All Results": self.results,
        }

        return summary

    def cleanup_resources(self):
        """Clean up any remaining resources in kind or remote cluster."""
        if self.parsed_inputs:
            _, yaml_file, _, _ = self.parsed_inputs
            self.logger.info(f"Deleting YAML file: {yaml_file}")
            # TODO: Implement cleanup for kind v remote v simulation.
            # delete_yaml(yaml_file)


if __name__ == "__main__":
    log_path = "my_custom_logger.log"
    benchmark = DualPodsBenchmark()

    # Run an example benchmark
    kwargs = {"num_model_variants": 3}
    benchmark.run_benchmark("Introducing New Variant", 3, **kwargs)
    benchmark.logger.info(benchmark.get_results())
