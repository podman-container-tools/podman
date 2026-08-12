#!/usr/bin/env python3

from html.parser import HTMLParser
import os
import sys

# The classes logformatter only ever emits for its ginkgo layout. Its bats
# layout marks failures with 'bats-' classes instead.
GINKGO_CLASS = 'log-failed'
GINKGO_CLASS_PREFIX = 'ginkgo-'

class GinkgoLogFilterParser(HTMLParser):
    def __init__(self):
        super().__init__()
        # Stack to keep track of nested elements
        self.stack = []
        # Store the raw HTML strings of matching 'tt' elements
        self.results = []
        # Set once a class only the ginkgo layout emits has been seen
        self.is_ginkgo = False

    def _get_classes(self, attrs):
        """Helper to extract classes from an attribute list."""
        for attr, value in attrs:
            if attr == 'class' and value:
                return value.split()
        return []

    def _detect_ginkgo(self, classes):
        """Note the classes logformatter only emits for the ginkgo layout."""
        if any(c == GINKGO_CLASS or c.startswith(GINKGO_CLASS_PREFIX)
               for c in classes):
            self.is_ginkgo = True

    def handle_starttag(self, tag, attrs):
        classes = self._get_classes(attrs)
        is_tt = 'tt' in classes
        is_failed = 'log-failed' in classes
        self._detect_ginkgo(classes)

        # If we see a 'log-failed' class, flag all 'tt' ancestors in the stack
        if is_failed:
            for node in self.stack:
                if node['is_tt']:
                    node['keep'] = True

        # Push the new element to the stack
        self.stack.append({
            'tag': tag,
            'text_chunks': [],
            'is_tt': is_tt,
            'keep': False
        })

    def handle_startendtag(self, tag, attrs):
        # Handle self-closing tags just to check for the failure class
        classes = self._get_classes(attrs)
        self._detect_ginkgo(classes)
        if 'log-failed' in classes:
            for node in self.stack:
                if node['is_tt']:
                    node['keep'] = True

    def handle_data(self, data):
        # Capture raw text data
        if self.stack:
            self.stack[-1]['text_chunks'].append(data)

    def handle_endtag(self, tag):
        # Find the matching start tag in the stack
        for i in reversed(range(len(self.stack))):
            if self.stack[i]['tag'] == tag:
                # Pop the matching tag and any unclosed children
                while len(self.stack) > i:
                    node = self.stack.pop()

                    # Join all the collected text for this node
                    node_text = "".join(node['text_chunks'])
                    # Trim down the spaces for the ginkgo long indentation.
                    node_text = node_text.removeprefix(' ' * 9)

                    # If this is a 'tt' element and it contains a 'log-failed' child, save the text
                    if node['is_tt'] and node['keep']:
                        self.results.append(node_text)

                    # Pass the text of this node up to its parent so we don't lose inner text
                    if self.stack:
                        self.stack[-1]['text_chunks'].append(node_text)
                break

class BatsLogFilterParser(HTMLParser):
    def __init__(self):
        super().__init__()
        # Current Tag which text should be stored
        self.record = None
        self.data = ""

    def _get_classes(self, attrs):
        """Helper to extract classes from an attribute list."""
        for attr, value in attrs:
            if attr == 'class' and value:
                return value.split()
        return []

    def handle_starttag(self, tag, attrs):
        classes = self._get_classes(attrs)

        if 'bats-failed' in classes or 'bats-log-failblock' in classes or 'bats-log' in classes:
            self.record = tag

    def handle_data(self, data):
        if self.record:
            self.data += data

    def handle_endtag(self, tag):
        if self.record == tag:
            self.data += "\n"
            self.record = None



def filter_html_file(file_path):
    # Read the HTML content. logformatter passes its input through ':utf8',
    # which does not validate, so a test that wrote raw bytes leaves us with a
    # log we must not choke on.
    with open(file_path, 'r', encoding='utf-8', errors='replace') as f:
        html_content = f.read()

    # logformatter picks the ginkgo or bats layout from the log contents, so
    # tell them apart the same way here. The file name is no guide: int is not
    # the only ginkgo suite, bindings is one too.
    ginkgo_parser = GinkgoLogFilterParser()
    ginkgo_parser.feed(html_content)
    if ginkgo_parser.is_ginkgo:
        return ginkgo_parser.results

    bats_parser = BatsLogFilterParser()
    bats_parser.feed(html_content)
    return [bats_parser.data]


def main(file_paths):
    if not file_paths:
        print(f"usage: {os.path.basename(sys.argv[0])} LOGFILE.html...", file=sys.stderr)
        return 2

    with_headings = len(file_paths) > 1

    for file_path in file_paths:
        try:
            matching_elements = filter_html_file(file_path)
        except OSError as e:
            # The caller passes a glob, which the shell hands over unexpanded
            # when a job produced no html log at all.
            print(f"skipping {file_path}: {e}", file=sys.stderr)
            continue

        if with_headings:
            print(f"### {os.path.basename(file_path)}")

        for element in matching_elements:
            if not element.strip():
                continue
            print("```")
            print(element)
            print("```")

    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
