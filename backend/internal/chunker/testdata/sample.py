import os
from pathlib import Path

# TODO: add logging support

class Calculator:
    """A simple calculator class."""

    def add(self, a, b):
        """Return the sum."""
        return a + b

    def _protected_method(self):
        """Internal helper."""
        pass

    def __private_method(self):
        """Truly private."""
        pass

def multiply(a, b):
    """Multiply two numbers."""
    # FIXME: validate input types
    return a * b

result = multiply(3, 4)
os.path.exists("/tmp")
Path("/tmp").resolve()
