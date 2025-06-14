package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	lst "github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const listHeight = 9
const defaultWidth = 20
const expression1 string = `├|─|│|└`
const expression2 string = `(?P<current>\*?)\s*(?P<num>[0-9]*)\. (?P<name>.*)\[(?P<vol>.*)\]`

var (
	// background = lipgloss.Color("#424242")
	background = lipgloss.Color("#3f3f3f")

	titleStyle        = lipgloss.NewStyle().MarginLeft(2).Foreground(lipgloss.Color("#efef8f")).Background(background)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4).Background(background)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("#7f9f7f")).Background(background)
	// paginationStyle   = lst.DefaultStyles().PaginationStyle.PaddingLeft(4).Background(background)
	// helpStyle         = lst.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1).Foreground(lipgloss.Color("#dcdccc"))
	helpStyle     = lst.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	quitTextStyle = lipgloss.NewStyle().Margin(1, 0, 2, 4).Foreground(lipgloss.Color("#efef8f")).Background(background)
	activeDot     = lipgloss.NewStyle().Foreground(lipgloss.Color("#cc9393")).SetString("•")
	inactiveDot   = lipgloss.NewStyle().Foreground(lipgloss.Color("#dfdfdf")).SetString("•")
)

// item refers to an audio sink (name and id) as reported by WirePlumber
type item struct {
	name, id string
}

func (i item) FilterValue() string { return "" }

type itemDelegate struct{}

func (d itemDelegate) Height() int                            { return 1 }
func (d itemDelegate) Spacing() int                           { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *lst.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m lst.Model, index int, listItem lst.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%d. %s", index+1, i)

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("> " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(str))
}

type model struct {
	list     lst.Model
	choice   string
	id       string
	quitting bool
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			i, ok := m.list.SelectedItem().(item)
			if ok {
				m.choice = string(i.name)
				m.id = string(i.id)
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// The event when an audio sink is selected, or exited without selecting anything
func (m model) View() string {
	if m.choice != "" {

		cmd := exec.Command("wpctl", "set-default", m.id)
		_, err := cmd.Output()
		if err != nil {
			errn := errors.New("Error executing 'wpctl set-default'")
			log.Fatal(errors.Join(errn, err))
		}
		return quitTextStyle.Render(fmt.Sprintf("Setting %s.", m.choice))
	}
	if m.quitting {
		return quitTextStyle.Render("Skipping for now...")
	}
	return "\n" + m.list.View()
}

// retrieve_sinks returns a list of audio sink names and id's as reported by
// WirePlumber (executing wpctl status)
func retrieve_sinks() (names, nums []string, err error) {
	var (
		key    int
		value  string
		nsinks uint8
		sinks  []string
	)

	nsinks = 6 // arbitrary; >= 6 audio sinks is surely rare

	cmd := exec.Command("wpctl", "status")
	out, err := cmd.Output()
	if err != nil {
		errn := errors.New("Error executing 'wpctl status'")
		return nil, nil, errors.Join(errn, err)
	}

	re1 := regexp.MustCompile(expression1)
	s1 := re1.ReplaceAllString(string(out), "")

	// find the location where the audio sinks printout starts
	// there is an assumption that the audio section is listed
	// before the video section
	lines := strings.Split(s1, "\n")
	for key, value = range lines {
		if strings.Contains(value, "Sinks:") {
			break
		}
	}

	sinks = make([]string, 0, nsinks)

	for _, value = range lines[key+1:] {
		sv := strings.TrimSpace(value)
		// check for end of the audio section
		if sv == "" {
			break
		}
		sinks = append(sinks, sv)
	}

	re2 := regexp.MustCompile(expression2)

	names = make([]string, 0, nsinks)
	nums = make([]string, 0, nsinks)

	for _, value = range sinks {
		match := re2.FindStringSubmatch(value)
		current := match[1]
		num := match[2]
		name := match[3]

		names = append(names, current+name)
		nums = append(nums, num)

	}

	return names, nums, nil
}

func tui(names, nums []string) model {
	items := make([]lst.Item, 0, 6)

	for k, v := range names {
		items = append(items, item{v, nums[k]})
	}

	hlp := help.Model{
		ShortSeparator: " • ",
		FullSeparator:  "    ",
		Ellipsis:       "…",
		Styles: help.Styles{
			// ShortKey:       lipgloss.NewStyle().Foreground(lipgloss.Color("#dfdfdf")),
			ShortKey:       lipgloss.NewStyle().Foreground(lipgloss.Color("#8cd0d3")).Background(background),
			ShortDesc:      lipgloss.NewStyle().Foreground(lipgloss.Color("#e3ceab")).Background(background),
			ShortSeparator: lipgloss.NewStyle().Foreground(lipgloss.Color("#dfdfdf")).Background(background),
			Ellipsis:       lipgloss.NewStyle().Foreground(lipgloss.Color("#dfdfdf")).Background(background),
			FullKey:        lipgloss.NewStyle().Foreground(lipgloss.Color("#dfdfdf")).Background(background),
			FullDesc:       lipgloss.NewStyle().Foreground(lipgloss.Color("#dcdccc")).Background(background),
			FullSeparator:  lipgloss.NewStyle().Foreground(lipgloss.Color("#5b605e")).Background(background),
		},
	}

	sink_lst := lst.New(items, itemDelegate{}, defaultWidth, listHeight)
	sink_lst.Title = "Select an audio sink"
	sink_lst.SetShowStatusBar(false)
	sink_lst.SetFilteringEnabled(false)
	sink_lst.Styles.Title = titleStyle
	// sink_lst.Styles.PaginationStyle = paginationStyle
	sink_lst.Styles.HelpStyle = helpStyle
	sink_lst.Help = hlp

	// sink_lst.Styles.NoItems.Foreground(lipgloss.Color("#fcba03"))
	sink_lst.Paginator.ActiveDot = activeDot.String()
	sink_lst.Paginator.InactiveDot = inactiveDot.String()

	fmt.Println(sink_lst.ShowHelp())

	tui_model := model{list: sink_lst}

	return tui_model
}

func main() {

	names, nums, err := retrieve_sinks()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(names, nums)
	tui_model := tui(names, nums)

	if _, err := tea.NewProgram(tui_model).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
