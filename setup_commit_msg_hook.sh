#!/bin/sh

cat >.git/hooks/commit-msg <<'EOF'
#!/bin/sh

grep "^License:" "$1" || {
        echo >>"$1"
        echo "License: MIT" >>"$1"
        echo "Signed-off-by: $(git config user.name) <$(git config user.email)>" >>"$1"
}
EOF
chmod +x .git/hooks/commit-msg

