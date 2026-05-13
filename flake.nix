{
  description = "OpenConnect browser authentication helper";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";

  outputs = { self, nixpkgs, ... }:
    let
      lib = nixpkgs.lib;

      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      goVersion =
        let
          goLines = lib.filter (lib.hasPrefix "go ") (lib.splitString "\n" (builtins.readFile ./go.mod));
        in
        lib.removePrefix "go " (builtins.head goLines);

      goAttr = "go_${lib.replaceStrings [ "." ] [ "_" ] goVersion}";

      forAllSystems = lib.genAttrs supportedSystems;

      pkgsFor = system: import nixpkgs {
        inherit system;
      };

      goFor = pkgs: pkgs.${goAttr};

      buildGoModuleFor = pkgs: pkgs.buildGoModule.override {
        go = goFor pkgs;
      };

      packageVersion = "unstable-${self.shortRev or "dirty"}";
    in
    {
      packages = forAllSystems (system:
        let
          pkgs = pkgsFor system;
          buildGoModule = buildGoModuleFor pkgs;
          runtimePath = lib.makeBinPath (
            [
              pkgs.openconnect
            ] ++ lib.optionals pkgs.stdenv.isLinux [
              pkgs.chromium
            ]
          );
        in
        {
          default = buildGoModule {
            pname = "ocgo";
            version = packageVersion;

            src = self;
            vendorHash = "sha256-IgT/gCA+yXcck1wFUUnsQADNwyot43r3r5my2l4e4oI=";

            ldflags = [
              "-s"
              "-w"
            ];

            nativeBuildInputs = [
              pkgs.makeWrapper
            ];

            postInstall = ''
              wrapProgram "$out/bin/ocgo" \
                --prefix PATH : "${runtimePath}"
            '';

            meta = {
              description = "OpenConnect browser authentication helper";
              homepage = "https://github.com/ppanconi/ocgo";
              license = pkgs.lib.licenses.mit;
              mainProgram = "ocgo";
              platforms = supportedSystems;
            };
          };
        });

      apps = forAllSystems (system: {
        default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/ocgo";
          meta.description = "Run ocgo";
        };
      });

      checks = forAllSystems (system: {
        default = self.packages.${system}.default;
      });

      devShells = forAllSystems (system:
        let
          pkgs = pkgsFor system;
        in
        {
          default = pkgs.mkShell {
            packages = [
              (goFor pkgs)
            ] ++ (with pkgs; [
              gopls
              golangci-lint
              openconnect
            ]) ++ pkgs.lib.optionals pkgs.stdenv.isLinux [
              pkgs.chromium
            ];
          };
        });
    };
}
