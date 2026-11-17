"""
Configure pytest
"""


def pytest_report_header(config):
    """Add header to report."""
    return "python client -- DockerClient"
