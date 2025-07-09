# Gud - A Simple Git-like Version Control System (Git Gud)

Gud is a lightweight, minimalistic version control system implemented in Go. 
It provides basic version control features to track changes in your projects without the complexity of full Git.

---

## Features

- Initialize a new Gud repository (`init`)
- Create and list commits (`commit`, `log`)
- Manage branches (`branch`, `checkout`)
- Stage files before committing (`add`)
- Push and pull commits to/from a remote repository (`push`, `pull`)
- Merge and rebase branches (`merge`, `rebase`)
- Clone repositories (`clone`)
- Revert to a specific commit (`revert`)
- Show file differences (`diff`)
- Show repository status (`status`)

---

## Installation

1. Make sure you have [Go](https://golang.org/dl/) installed.
2. Clone this repository and build:

```bash
git clone https://github.com/EonaCat/gud.git
cd gud
go build -o gud
```

Move the gud binary to your PATH or run it directly.

### Usage

Initialize a new repository:

```bash
gud init
```

Add files to staging area:

```bash
gud add <filename>
```

Create a commit with a message:

```bash
gud commit -m "Your commit message"
```

View commit history:

```bash
gud log
```

Create a new branch:

```bash
gud branch <branch-name>
```

Switch to a branch:

```bash
gud checkout <branch-name>
```

Push commits to remote:

```bash
gud push
```

Pull commits from remote:

``` bash
gud pull
```

Merge a branch into the current branch:

```bash
gud merge <branch-name>
```

Rebase a branch onto another:

```bash
gud rebase <base-branch> <target-branch>
```

Clone a remote repository:

```bash
gud clone <remote-path> <target-directory>
```

Revert working directory to a specific commit:

```bash
gud revert <commit-id>
```

Show differences between working directory and last commit:

```bash
gud diff
```

Show repository status:

```bash
gud status
```



## Repository Structure

Gud stores its internal data in the .gud directory inside your project root, containing:
commits/ - JSON files representing commits
branches/ - Current branch pointers
staging/ - Staged files snapshot
HEAD - Current branch reference
remote_url - Remote repository location
logs/ - Commit logs

## Limitations

No network communication; remote operations work by copying files locally.
No conflict detection during merge or rebase.
No advanced Git features like tags, stash, hooks, etc.
Designed for learning and experimentation, not production use.

## Contributing
Contributions are welcome! Please open issues or submit pull requests for bugs or new features.