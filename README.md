![container-up logo](assets/container-up.png)
============

`container-up` is a tool for updating docker container.
This tool recreate container with same configurations (volume mount, port mapping, and so on).

## update container
`$ container-up CONTAINER_NAME/ID`

## update container (with copy file)
`$ container-up -f /path/to/file.txt CONTAINER_NAME/ID`

### with multiple files
`$ container-up -f /path/to/file1.txt -f /path/to/file2.txt CONTAINER_NAME/ID`

## remove old container after update
`$ container-up --rm CONTAINER_NAME/ID`

## show help
`$ container-up --help`
