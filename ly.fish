function ly
    if test (count $argv) -eq 0
        layouts list
        return
    end

    set -l subcmd $argv[1]
    set -l rest $argv[2..]

    switch $subcmd
        case a apply
            layouts apply $rest
        case ls l list
            layouts list $rest
        case s show
            layouts show $rest
        case n new
            layouts new $rest
        case g grid
            layouts grid $rest
        case c cfg config
            layouts config $rest
        case i init
            layouts init $rest
        case '*'
            layouts $argv
    end
end
