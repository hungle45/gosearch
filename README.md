# Go Package Search CLI

A lightweight command-line tool written in Go for fuzzy searching Go packages from [index.golang.org](https://index.golang.org/) and copying their import paths to the clipboard.

## Features

* **Fuzzy Search:** Quickly find packages by typing.
* **Interactive Selection:** Navigate results with arrow keys.
* **Version Display:** Shows the latest package version.

## Installation

### Prerequisites

* Go (version 1.16+ recommended)
* For Linux: `xclip` (`sudo apt-get install xclip` or `sudo yum install xclip`)

### Steps

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/hungle45/gosearch
    cd gosearch
    ```

2.  **Initialize and get dependencies:**
    ```bash
    go mod tidy
    ```

3.  **Install the command:**
    From your project's root directory:
    ```bashCaught panic:

    go install .
    ```
    This compiles and places the executable `gosearch` into your `$GOPATH/bin` (or `$GOBIN`).

4.  **Verify `PATH` (if needed):**
    Ensure `$GOPATH/bin` (or `$GOBIN`) is in your system's `PATH`. This is usually automatic. If not, add it to your shell config (e.g., `~/.bashrc`):
    ```bash
    export PATH=$PATH:$(go env GOPATH)/bin
    ```
    Then, `source ~/.bashrc` (or your relevant file).

## Usage

### Running the application

* **From source (for quick testing):**
    ```bash
    go run main.go
    ```
* **Using the installed command (recommended after installation):**
    ```bash
    gosearch
