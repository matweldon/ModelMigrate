# ~/.bashrc

# Enable color support for ls and add handy aliases
if [ -x /usr/bin/dircolors ]; then
    eval "$(dircolors -b)"
    alias ls='ls --color=auto'
    alias grep='grep --color=auto'
fi

# Nicer prompt: user@host:cwd (git branch) $
parse_git_branch() {
    git branch 2>/dev/null | grep '^*' | colrm 1 2 | sed 's/.*/ (\x1b[35m&\x1b[0m)/'
}
export PS1="\[\e[32m\]\u@\h\[\e[0m\]:\[\e[34m\]\w\[\e[0m\]\[\e[33m\]\$(parse_git_branch)\[\e[0m\] \n \$ "

# Useful aliases
alias ll='ls -alF'
alias la='ls -A'
alias l='ls -CF'

# History settings
HISTCONTROL=ignoredups:erasedups
HISTSIZE=10000
HISTFILESIZE=20000
shopt -s histappend

# Enable bash completion if available
if [ -f /etc/bash_completion ]; then
    . /etc/bash_completion
fi
