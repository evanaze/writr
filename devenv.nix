{...}: {
  # https://devenv.sh/languages/
  languages.go.enable = true;
  languages.javascript = {
    enable = true;
    npm.enable = true;
  };

  # https://devenv.sh/scripts/
  scripts.build.exec = "rm writr && go build -o writr .";

  # https://devenv.sh/tests/
  enterTest = "CGO_ENABLED=0 go test ./...";
}
