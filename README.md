# DI Container

This package provides a simple dependency injection (DI) container for
 Go applications. It helps you manage the lifecycle of your dependencies,
 including setup and cleanup operations.


## Example

[You can simply play around it](./di_test.go#L42).


## Features

* **Dependency registration:** Register dependencies with their setup functions.
* **Named dependencies:**  Register dependencies with custom names to avoid conflicts.
* **Generic types:**  Use generics to define dependencies with specific types.
* **Setup and cleanup:** Define setup and cleanup functions for each dependency.
* **Options:** Customize dependency behavior with options like `OptNoReuse` and `OptMiddleware`.
* **Error handling:**  Handles errors during setup and cleanup.


## Installation

```sh
go get github.com/irr123/di
```
