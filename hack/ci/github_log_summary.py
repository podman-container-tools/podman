#!/usr/bin/env python

from html.parser import HTMLParser
import html
import sys

class GinkgoLogFilterParser(HTMLParser):
    def __init__(self):
        super().__init__()
        # Stack to keep track of nested elements
        self.stack = []
        # Store the raw HTML strings of matching 'tt' elements
        self.results = []

    def _get_classes(self, attrs):
        """Helper to extract classes from an attribute list."""
        for attr, value in attrs:
            if attr == 'class' and value:
                return value.split()
        return []

    def handle_starttag(self, tag, attrs):
        classes = self._get_classes(attrs)
        is_tt = 'tt' in classes
        is_failed = 'log-failed' in classes

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
    # Read the HTML content
    with open(file_path, 'r', encoding='utf-8') as f:
        html_content = f.read()

    if 'int-' in file_path:
        parser = GinkgoLogFilterParser()
        parser.feed(html_content)
        return parser.results

    parser = BatsLogFilterParser()
    parser.feed(html_content)
    return [parser.data]


# Running the filter
matching_elements = filter_html_file(sys.argv[1])

for element in matching_elements:
    print(f"```")
    print(element)
    print("```")
