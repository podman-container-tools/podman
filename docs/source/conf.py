# Configuration file for the Sphinx documentation builder.
#
# This file only contains a selection of the most common options. For a full
# list see the documentation:
# https://www.sphinx-doc.org/en/master/usage/configuration.html

# -- Path setup --------------------------------------------------------------

# If extensions (or modules to document with autodoc) are in another directory,
# add these directories to sys.path here. If the directory is relative to the
# documentation root, use os.path.abspath to make it absolute, like shown here.
#
# import os
# import sys
# sys.path.insert(0, os.path.abspath('.'))

import re
import os
import subprocess

# Define the canonical URL for our custom docs.podman.io domain configured on Read the Docs
html_baseurl = os.environ.get("READTHEDOCS_CANONICAL_URL", "")

# Tell Jinja2 templates the build is running on Read the Docs
if os.environ.get("READTHEDOCS", "") == "True":
    if "html_context" not in globals():
        html_context = {}
    html_context["READTHEDOCS"] = True

# We have to run the preprocessor to create the actual markdown files from .in files.
# Do it here so the it can work on readthedocs as well.
path = os.path.join(os.path.abspath(os.path.dirname(
    __file__)), "../../hack/markdown-preprocess")
p = subprocess.Popen(path,
                     stdout=subprocess.PIPE, stderr=subprocess.PIPE)
out, err = p.communicate()
if p.returncode != 0:
    raise Exception("failed to run markdown-preprocess", out, err)


# -- Project information -----------------------------------------------------

project = "Podman"
copyright = "2019, team"
author = "team"


# -- General configuration ---------------------------------------------------

# Add any Sphinx extension module names here, as strings. They can be
# extensions coming with Sphinx (named 'sphinx.ext.*') or your custom
# ones.
extensions = ["myst_parser"]

# Add any paths that contain templates here, relative to this directory.
templates_path = ["_templates"]

# List of patterns, relative to source directory, that match files and
# directories to ignore when looking for source files.
# This pattern also affects html_static_path and html_extra_path.
exclude_patterns = ["markdown/options"]

master_doc = "index"

# Configure smartquotes to only transform quotes and ellipses, not dashes
smartquotes_action = "qe"

locale_dirs = ["locale/"]

# -- Options for HTML output -------------------------------------------------

# The theme to use for HTML and HTML Help pages.  See the documentation for
# a list of builtin themes.
#
html_theme = "alabaster"

# Add any paths that contain custom static files (such as style sheets) here,
# relative to this directory. They are copied after the builtin static files,
# so a file named "default.css" will overwrite the builtin "default.css".
html_static_path = ["_static"]

html_css_files = [
    "custom.css",
]

# -- Extension configuration -------------------------------------------------

# IMPORTANT: explicitly unset the extensions, by default dollarmath is enabled.
# We use the dollar sign as text and do not want it to be interpreted as math expression.
myst_enable_extensions = []


def convert_markdown_title(app, docname, source):
    # Process markdown files only
    docpath = app.env.doc2path(docname)
    if docpath.endswith(".md"):
        # Convert pandoc title line into eval_rst block for myst_parser
        #
        # Remove the ending " 1" (section) to avoid it from being displayed
        # in the web tab. Often such a text indicates that
        # a web page got an update. For instance GitHub issues
        # shows the number of new comments that have been written
        # after the user's last visit.
        source[0] = re.sub(r"^% (.*)\s(\d)", r"```{title} \g<1>\n```", source[0])

def setup(app):
    app.connect("source-read", convert_markdown_title)
