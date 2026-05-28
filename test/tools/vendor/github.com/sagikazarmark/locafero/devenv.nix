{ pkgs, ... }:

{
  cachix.pull = [ "sagikazarmark-dev" ];

  languages = {
    go = {
      enable = true;
      package = pkgs.go_1_25;
    };
  };

  packages = with pkgs; [
    just
    golangci-lint
  ];
}
