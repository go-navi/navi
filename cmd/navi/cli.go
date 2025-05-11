package navi

import (
	"fmt"
	"runtime"
	"sort"
	"strings"

	"github.com/go-navi/navi/internal/logger"
	"github.com/go-navi/navi/internal/term"
	"github.com/go-navi/navi/internal/utils"
	"github.com/goccy/go-yaml"
	"github.com/kballard/go-shellquote"
)

// Constants for menu item types
const (
	COMMAND_TYPE = iota
	PROJ_COMMAND_TYPE
	RUNNER_TYPE
)

// NaviMenuItem represents an item in the CLI menu (command or runner)
type NaviMenuItem struct {
	Id      string // Unique identifier for the menu item
	Type    int    // Type of menu item ("command", "projectCommand" or "runner")
	Name    string // Display name (project name for commands, runner name for runners)
	CmdName string // Command name (only used for commands)
}

// State tracks the current state of the CLI interface
type State struct {
	SearchQuery             string // Current search filter text
	SelectedItemId          string // ID of currently selected item
	TermWidth               int    // Terminal width in characters
	TermHeight              int    // Terminal height in characters
	SelectedMenuItemIndex   int    // Index of selected item in filtered items
	TotalFilteredMenuItems  int    // Total number of items after filtering
	ListScrollOffset        int    // Vertical scroll position of list panel
	DefScrollOffset         int    // Vertical scroll position of definition panel
	ListPanelWidth          int    // Width of left panel showing commands/runners
	DefPanelWidth           int    // Width of right panel showing definitions
	CommandModalInputHeight int    // Height of command arguments input modal
	IsDefPanelActive        bool   // Whether definition panel is currently active
	OnCommandArgsModal      bool   // Whether command arguments modal is open
	MoreListItemsAbove      bool   // Whether there are more list items above current view
	MoreListItemsBelow      bool   // Whether there are more list items below current view
	MoreDefAbove            bool   // Whether there is more definition content above current view
	MoreDefBelow            bool   // Whether there is more definition content below current view
	CommandArgs             string // Current command arguments text
}

var (
	state, prevState       State          // Current and previous UI state
	allMenuItems           []NaviMenuItem // All available menu items
	filteredMenuItems      []NaviMenuItem // Menu items matching current filter
	selectedItemDefinition []string       // Definition lines for selected item
	commandArgModalLines   []string       // Lines of text for command arguments modal
	commandArgsModalWidth  int            // Width of command arguments modal
)

// HasChanges checks if current state differs from previous state
func (s *State) HasChanges() bool {
	return s.SearchQuery != prevState.SearchQuery ||
		s.SelectedItemId != prevState.SelectedItemId ||
		s.TermWidth != prevState.TermWidth ||
		s.TermHeight != prevState.TermHeight ||
		s.SelectedMenuItemIndex != prevState.SelectedMenuItemIndex ||
		s.TotalFilteredMenuItems != prevState.TotalFilteredMenuItems ||
		s.ListScrollOffset != prevState.ListScrollOffset ||
		s.DefScrollOffset != prevState.DefScrollOffset ||
		s.ListPanelWidth != prevState.ListPanelWidth ||
		s.CommandModalInputHeight != prevState.CommandModalInputHeight ||
		s.DefPanelWidth != prevState.DefPanelWidth ||
		s.IsDefPanelActive != prevState.IsDefPanelActive ||
		s.OnCommandArgsModal != prevState.OnCommandArgsModal ||
		s.MoreListItemsAbove != prevState.MoreListItemsAbove ||
		s.MoreListItemsBelow != prevState.MoreListItemsBelow ||
		s.MoreDefAbove != prevState.MoreDefAbove ||
		s.MoreDefBelow != prevState.MoreDefBelow ||
		s.CommandArgs != prevState.CommandArgs
}

// UpdateState copies current state to previous state
func (s *State) UpdateState() {
	prevState.SearchQuery = s.SearchQuery
	prevState.SelectedItemId = s.SelectedItemId
	prevState.TermWidth = s.TermWidth
	prevState.TermHeight = s.TermHeight
	prevState.SelectedMenuItemIndex = s.SelectedMenuItemIndex
	prevState.TotalFilteredMenuItems = s.TotalFilteredMenuItems
	prevState.ListScrollOffset = s.ListScrollOffset
	prevState.DefScrollOffset = s.DefScrollOffset
	prevState.ListPanelWidth = s.ListPanelWidth
	prevState.CommandModalInputHeight = s.CommandModalInputHeight
	prevState.DefPanelWidth = s.DefPanelWidth
	prevState.IsDefPanelActive = s.IsDefPanelActive
	prevState.OnCommandArgsModal = s.OnCommandArgsModal
	prevState.MoreListItemsAbove = s.MoreListItemsAbove
	prevState.MoreListItemsBelow = s.MoreListItemsBelow
	prevState.MoreDefAbove = s.MoreDefAbove
	prevState.MoreDefBelow = s.MoreDefBelow
	prevState.CommandArgs = s.CommandArgs
}

// highlightMatches returns a string with matching characters highlighted
func highlightMatches(text, filter string) string {
	if strings.TrimSpace(text) == "" || strings.TrimSpace(filter) == "" {
		return ""
	}

	result := make([]rune, len([]rune(text)))

	for i := range result {
		result[i] = ' '
	}

	lowerText := strings.ToLower(text)
	filterTerms := strings.Fields(strings.ToLower(filter))
	textRunes := []rune(text)

	for _, term := range filterTerms {
		termRunes := []rune(term)
		pos := 0

		for {
			index := strings.Index(lowerText[pos:], term)
			if index == -1 {
				break
			}

			actualPos := pos + index

			for i := 0; i < len(termRunes); i++ {
				if actualPos+i < len(result) {
					result[actualPos+i] = textRunes[actualPos+i]
				}
			}

			pos += index + len(term)

			if pos >= len(lowerText) {
				break
			}
		}
	}

	return string(result)
}

// stripFlags removes flags from runner name (e.g., "[serial]")
func stripFlags(runnerName string) string {
	bracketIdx := strings.LastIndex(runnerName, "[")

	if bracketIdx > 0 && strings.HasSuffix(runnerName, "]") {
		return runnerName[:bracketIdx]
	}

	return runnerName
}

// getKeyPriority returns sort priority for configuration keys
func getKeyPriority(key string) int {
	orderedKeys := []string{
		"dir",
		"shell",
		"watch",
		"include",
		"exclude",
		"dotenv",
		"env",
		"pre",
		"run",
		"post",
		"after",
		"success",
		"failure",
		"change",
		"always",
		"cmds",
		"cmd",
		"name",
		"serial",
		"dependent",
		"delay",
		"restart",
		"retries",
		"interval",
		"condition",
		"awaits",
		"ports",
		"timeout",
	}

	for i, orderedKey := range orderedKeys {
		if key == orderedKey {
			return i
		}
	}

	return len(orderedKeys)
}

// formatConfigData transforms configuration data into formatted display lines
func formatConfigData(
	data any,
	indent int,
	itemType int,
	parentKey string,
) (defLines []string) {
	termUI := term.NewTermUI()
	indentSpace := "  "

	// Add ellipsis divider for "cmds" field
	if itemType == PROJ_COMMAND_TYPE && parentKey == "cmds" {
		defLines = append(
			defLines,
			termUI.
				Store().
				Repeat(indentSpace, indent).
				DarkGray().Print("...").PrevColor().
				GetStored(),
		)
	}

	switch v := data.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))

		for k := range v {
			keys = append(keys, k)
		}

		if parentKey == "cmds" ||
			parentKey == "commands" ||
			parentKey == "projects" ||
			parentKey == "runners" {
			sort.Strings(keys)
		} else {
			sort.Slice(keys, func(i, j int) bool {
				iPriority := getKeyPriority(keys[i])
				jPriority := getKeyPriority(keys[j])

				if iPriority != jPriority {
					return iPriority < jPriority
				}

				return keys[i] < keys[j]
			})
		}

		for _, k := range keys {
			val := v[k]

			switch val.(type) {
			case map[string]any, []any:
				defLines = append(
					defLines,
					termUI.
						Store().
						Repeat(indentSpace, indent).
						Cyan().Print(k).PrevColor().Print(":").
						GetStored(),
				)

				valLines := formatConfigData(val, indent+1, itemType, k)
				defLines = append(defLines, valLines...)

			default:
				valLines := formatConfigData(val, indent+1, itemType, k)

				if len(valLines) == 1 {
					defLines = append(
						defLines,
						termUI.
							Store().
							Repeat(indentSpace, indent).
							Cyan().Print(k).PrevColor().Print(": "+strings.TrimSpace(valLines[0])).
							GetStored(),
					)
				} else {
					defLines = append(
						defLines,
						termUI.
							Store().
							Repeat(indentSpace, indent).
							Cyan().Print(k).PrevColor().Print(":").
							GetStored(),
					)

					defLines = append(defLines, valLines...)
				}
			}
		}

	case []any:
		for _, item := range v {
			itemLines := formatConfigData(item, indent+1, itemType, parentKey)

			if len(itemLines) > 0 {
				for idx, line := range itemLines {
					termUI.Store()

					if idx == 0 {
						termUI.
							Repeat(indentSpace, indent).
							Print("- " + strings.TrimSpace(line))
					} else {
						termUI.Print(line)
					}

					newLine := termUI.GetStored()
					defLines = append(defLines, newLine)
				}
			}
		}

	default:
		if utils.IsNumber(v) {
			defLines = append(
				defLines,
				termUI.
					Store().
					Repeat(indentSpace, indent).
					Yellow().Print(v).PrevColor().
					GetStored(),
			)
		} else if utils.IsBool(v) {
			defLines = append(
				defLines,
				termUI.
					Store().
					Repeat(indentSpace, indent).
					Green().Print(v).PrevColor().
					GetStored(),
			)
		} else {
			if strVal, ok := v.(string); ok {
				strLines := strings.Split(strVal, "\n")

				for _, line := range strLines {
					if strings.TrimSpace(line) == "" {
						continue
					}

					defLines = append(
						defLines,
						termUI.
							Store().
							Repeat(indentSpace, indent).
							Print(line).
							GetStored(),
					)
				}
			} else {
				defLines = append(
					defLines,
					termUI.
						Store().
						Repeat(indentSpace, indent).
						Print(v).
						GetStored(),
				)
			}
		}
	}

	return defLines
}

// renderTitle draws the application title at the top of the screen
func renderTitle() {
	termUI := term.NewTermUI()

	resized := state.TermWidth != prevState.TermWidth ||
		state.TermHeight != prevState.TermHeight

	modalClosed := prevState.OnCommandArgsModal && !state.OnCommandArgsModal
	modalShrank := prevState.CommandModalInputHeight > state.CommandModalInputHeight

	if resized || modalClosed || modalShrank {
		termUI.
			Cursor(1, 1).
			ClearLine().
			Repeat(" ", state.ListPanelWidth-3).
			Cyan().
			Print("Navi CLI").
			PrevColor()
	}
}

// renderSeparators draws separator lines and panel dividers
func renderSeparators() {
	termUI := term.NewTermUI()

	resized := state.TermWidth != prevState.TermWidth ||
		state.TermHeight != prevState.TermHeight

	modalClosed := prevState.OnCommandArgsModal && !state.OnCommandArgsModal
	modalShrank := prevState.CommandModalInputHeight > state.CommandModalInputHeight

	if resized || modalClosed || modalShrank {
		// draw title separator
		termUI.
			Cursor(1, 2).
			ClearLine().
			DarkGray().
			Repeat("-", state.TermWidth).
			PrevColor()

		// draw panel separator
		titleHeight := 2
		commandHintsHeight := 2
		separatorHeight := state.TermHeight - titleHeight - commandHintsHeight
		separatorX := state.ListPanelWidth + 2 // 2 => 1 for padding + 1 for separator
		separatorY := 3                        // starts after title and titleSeparator

		termUI.
			// clear padding on left
			Cursor(separatorX-1, separatorY).
			RepeatVertical(" ", separatorHeight).
			// draw separator
			Cursor(separatorX, separatorY).
			DarkGray().
			RepeatVertical("¦", separatorHeight).
			PrevColor().
			// clear padding on right
			Cursor(separatorX+1, separatorY).
			RepeatVertical(" ", separatorHeight)

		// draw hints separator
		termUI.
			Cursor(1, state.TermHeight-1).
			ClearLine().
			DarkGray().
			Repeat("-", state.TermWidth).
			PrevColor()
	}
}

// getSelectedMenuItem returns the currently selected menu item or nil
func getSelectedMenuItem() *NaviMenuItem {
	if state.SelectedMenuItemIndex < 0 || state.SelectedMenuItemIndex >= len(filteredMenuItems) {
		return nil
	}

	return &filteredMenuItems[state.SelectedMenuItemIndex]
}

// getPanelContentHeight calculates available height for panel content
func getPanelContentHeight() int {
	panelContentHeight := state.TermHeight - 7 // - 7 = hints + separator + title + separator + arrow above + arrow below + panel title

	if panelContentHeight < 0 {
		panelContentHeight = 0
	}

	return panelContentHeight
}

// renderHints displays keyboard shortcut help at the bottom
func renderHints() {
	termUI := term.NewTermUI()

	resized := state.TermWidth != prevState.TermWidth ||
		state.TermHeight != prevState.TermHeight

	modalChanged := state.OnCommandArgsModal != prevState.OnCommandArgsModal
	modalShrank := prevState.CommandModalInputHeight > state.CommandModalInputHeight
	selectedChanged := state.SelectedItemId != prevState.SelectedItemId
	searchChanged := state.SearchQuery != prevState.SearchQuery

	if resized || modalChanged || modalShrank || searchChanged || selectedChanged {
		termUI.Cursor(1, state.TermHeight).ClearLine()

		if !state.OnCommandArgsModal {
			hints := ""

			selectedItem := getSelectedMenuItem()
			if selectedItem != nil {
				if runtime.GOOS == "darwin" { // MacOS
					hints += "[Return] Execute   "
				} else {
					hints += "[Enter] Execute   "
				}

				if selectedItem.Type == COMMAND_TYPE || selectedItem.Type == PROJ_COMMAND_TYPE {
					hints += "[Ctrl+Space] Execute w/ args   "
				}
			}

			if state.MoreDefAbove || state.MoreDefBelow {
				if state.IsDefPanelActive {
					hints += "[Tab] Navigate commands   "
				} else {
					hints += "[Tab] Navigate definition   "
				}
			}

			if strings.TrimSpace(state.SearchQuery) != "" {
				hints += "[Esc] Clear filter"
			} else {
				hints += "[Esc] Exit"
			}

			termUI.
				ClipToWidth(state.TermWidth).
				DarkGray().
				Print(strings.TrimSpace(hints)).
				PrevColor().
				ApplyTextLimits()
		}
	}
}

// renderItems draws menu items in the left panel
func renderItems(topLine, totalLines int, fullRender bool) {
	termUI := term.NewTermUI()
	startIdx := state.ListScrollOffset

	for i := 0; i < totalLines; i++ {
		itemIdx := startIdx + i

		if fullRender {
			termUI.Cursor(1, topLine+i).Repeat(" ", state.ListPanelWidth)
		}

		if itemIdx < len(filteredMenuItems) {
			menuItem := filteredMenuItems[itemIdx]

			var typeText string
			switch menuItem.Type {
			case COMMAND_TYPE:
				typeText = "Command"
			case PROJ_COMMAND_TYPE:
				typeText = "Project"
			case RUNNER_TYPE:
				typeText = "Runner"
			}

			itemName := menuItem.Name
			projectCommand := ""

			if menuItem.Type == PROJ_COMMAND_TYPE {
				projectCommand = ":" + menuItem.CmdName
				itemName += projectCommand
			}

			itemNameWidth := state.ListPanelWidth - term.CountPrintableChars(typeText)

			if itemNameWidth < 0 {
				itemNameWidth = 0
			}

			itemNamePadding := itemNameWidth - term.CountPrintableChars(itemName)

			if itemNamePadding < 0 {
				itemNamePadding = 0
			}

			printSelected := func() {
				termUI.
					Cursor(1, topLine+i).
					ReverseVideo().
					ClipToWidth(itemNameWidth).
					Print(itemName).
					ApplyTextLimits().
					Repeat(" ", itemNamePadding).
					Print(typeText).
					PrevColor()
			}

			printNormal := func() {
				termUI.
					Cursor(1, topLine+i).
					ClipToWidth(itemNameWidth).
					Cyan().
					Print(menuItem.Name).
					PrevColor().
					Print(projectCommand). // empty if not a project command
					ApplyTextLimits().
					Repeat(" ", itemNamePadding).
					Print(typeText)

				// highlight search matches
				highlightedText := highlightMatches(itemName, state.SearchQuery)

				if term.CountPrintableChars(highlightedText) > 0 {
					termUI.
						Cursor(1, topLine+i).
						ClipToWidth(itemNameWidth).
						Yellow()

					for _, r := range highlightedText {
						if string(r) == " " {
							termUI.MoveCursorRight()
						} else {
							termUI.Print(string(r))
						}
					}

					termUI.
						PrevColor().
						ApplyTextLimits()
				}
			}

			switch {
			case itemIdx == state.SelectedMenuItemIndex:
				printSelected()
			case fullRender:
				printNormal()
			case itemIdx == prevState.SelectedMenuItemIndex:
				printNormal()
			}
		}
	}

	if state.TotalFilteredMenuItems == 0 {
		termUI.
			Cursor(1, topLine).
			Repeat(" ", state.ListPanelWidth).
			Cursor(1, topLine).
			BoldYellow().
			ClipToWidth(state.ListPanelWidth).
			Print("No matching items found").
			ApplyTextLimits().
			PrevColor()
	}
}

// renderListPanel draws the left panel with menu items and search
func renderListPanel() {
	termUI := term.NewTermUI()

	resized := state.TermWidth != prevState.TermWidth ||
		state.TermHeight != prevState.TermHeight

	modalClosed := prevState.OnCommandArgsModal && !state.OnCommandArgsModal
	modalShrank := prevState.CommandModalInputHeight > state.CommandModalInputHeight
	scrollOffsetChanged := state.ListScrollOffset != prevState.ListScrollOffset
	selectedMenuItemIndexChanged := state.SelectedMenuItemIndex != prevState.SelectedMenuItemIndex
	totalFilteredMenuItemsChanged := state.TotalFilteredMenuItems != prevState.TotalFilteredMenuItems
	searchQueryChanged := strings.TrimSpace(state.SearchQuery) != strings.TrimSpace(prevState.SearchQuery)

	// render more items above arrow
	moreAboveChanged := state.MoreListItemsAbove != prevState.MoreListItemsAbove
	activePanelChanged := state.IsDefPanelActive != prevState.IsDefPanelActive
	moreAboveLine := 4

	if resized || modalClosed || modalShrank || moreAboveChanged {
		termUI.Cursor(1, moreAboveLine).Repeat(" ", state.ListPanelWidth)
	}

	if state.MoreListItemsAbove && (resized || modalClosed || modalShrank || activePanelChanged || moreAboveChanged) {
		termUI.Cursor(state.ListPanelWidth/2, moreAboveLine)

		if !state.IsDefPanelActive { // list panel is active
			termUI.BoldYellow()
		} else {
			termUI.DarkGray()
		}

		termUI.Print("↑").PrevColor()
	}

	// render menu items
	itemsStartingLine := moreAboveLine + 1
	panelContentHeight := getPanelContentHeight()

	if resized || modalClosed || modalShrank || scrollOffsetChanged || searchQueryChanged || totalFilteredMenuItemsChanged {
		renderItems(itemsStartingLine, panelContentHeight, true)
	} else if selectedMenuItemIndexChanged {
		renderItems(itemsStartingLine, panelContentHeight, false)
	}

	// render more items below arrow
	moreBelowChanged := state.MoreListItemsBelow != prevState.MoreListItemsBelow
	moreBelowLine := state.TermHeight - 2

	if resized || modalClosed || modalShrank || moreBelowChanged {
		termUI.Cursor(1, moreBelowLine).Repeat(" ", state.ListPanelWidth)
	}

	if state.MoreListItemsBelow && (resized || modalClosed || modalShrank || activePanelChanged || moreBelowChanged) {
		termUI.Cursor(state.ListPanelWidth/2, moreBelowLine)

		if !state.IsDefPanelActive { // list panel is active
			termUI.BoldYellow()
		} else {
			termUI.DarkGray()
		}

		termUI.Print("↓").PrevColor()
	}

	// render filter header and position cursor
	filterLabel := "Filter: "

	if resized || modalClosed || modalShrank {
		termUI.Cursor(1, 3).Print(filterLabel)
	}

	selectedItemPos := state.SelectedMenuItemIndex + 1

	if state.TotalFilteredMenuItems == 0 {
		selectedItemPos = 0
	}

	inputInitialX := term.CountPrintableChars(filterLabel) + 1
	filterCounterText := termUI.
		Store().
		Print("[").
		Yellow().
		Print(selectedItemPos).
		PrevColor().
		Print(" of ").
		Yellow().
		Print(state.TotalFilteredMenuItems).
		PrevColor().
		Print("]").
		GetStored()

	searchInputWidth := state.ListPanelWidth -
		term.CountPrintableChars(filterLabel) - 1 - // 1 = filter counter padding
		term.CountPrintableChars(filterCounterText)

	if searchInputWidth < 0 {
		searchInputWidth = 0
	}

	termUI.Cursor(inputInitialX, 3)

	fullSearchQueryChanged := state.SearchQuery != prevState.SearchQuery

	if resized || modalClosed || modalShrank || selectedMenuItemIndexChanged || totalFilteredMenuItemsChanged || fullSearchQueryChanged {
		termUI.Repeat(" ", searchInputWidth)
	}

	if resized || modalClosed || modalShrank || selectedMenuItemIndexChanged || totalFilteredMenuItemsChanged {
		termUI.Cursor(inputInitialX+searchInputWidth, 3).Print(" " + filterCounterText)
	}

	displaySearchQuery := state.SearchQuery

	if term.CountPrintableChars(displaySearchQuery) > searchInputWidth {
		displaySearchQuery = displaySearchQuery[term.CountPrintableChars(displaySearchQuery)-(searchInputWidth):]
	}

	if resized || modalClosed || modalShrank || selectedMenuItemIndexChanged || totalFilteredMenuItemsChanged || fullSearchQueryChanged {
		termUI.Cursor(inputInitialX, 3).BoldGreen().Print(displaySearchQuery).PrevColor()
	} else {
		termUI.Cursor(inputInitialX+term.CountPrintableChars(displaySearchQuery), 3)
	}
}

// renderDefContent draws definition content with proper scrolling
func renderDefContent(panelX, topLine, totalLines int, defLines []string, errMsg string) {
	termUI := term.NewTermUI()
	startLineIdx := state.DefScrollOffset

	if startLineIdx+totalLines > len(defLines) {
		startLineIdx = len(defLines) - totalLines
	}

	if startLineIdx < 0 {
		startLineIdx = 0
	}

	for i := 0; i < totalLines; i++ {
		lineIndex := startLineIdx + i
		termUI.Cursor(panelX, topLine+i).ClearLine()

		if lineIndex < len(defLines) {
			defLine := defLines[lineIndex]

			termUI.
				Cursor(panelX, topLine+i).
				ClipToWidth(state.DefPanelWidth).
				Print(defLine).
				ApplyTextLimits()
		}
	}

	if errMsg != "" {
		termUI.
			Cursor(panelX, topLine).
			ClearLine().
			BoldYellow().
			ClipToWidth(state.DefPanelWidth).
			Print(errMsg).
			ApplyTextLimits().
			PrevColor()
	}
}

// renderDefinitionPanel draws the right panel with item definition
func renderDefinitionPanel() {
	termUI := term.NewTermUI()
	itemDefLines, errMsg := getSelectedItemDefinition()

	resized := state.TermWidth != prevState.TermWidth ||
		state.TermHeight != prevState.TermHeight

	modalClosed := prevState.OnCommandArgsModal && !state.OnCommandArgsModal
	modalShrank := prevState.CommandModalInputHeight > state.CommandModalInputHeight
	scrollOffsetChanged := state.DefScrollOffset != prevState.DefScrollOffset
	panelLeftX := state.TermWidth - (state.DefPanelWidth - 1) // - 1 = definition padding left

	if resized || modalClosed || modalShrank {
		termUI.Cursor(panelLeftX, 3).ClearLine().Print("Definition:")
	}

	// render more definition lines above arrow
	moreAboveChanged := state.MoreDefAbove != prevState.MoreDefAbove
	activePanelChanged := state.IsDefPanelActive != prevState.IsDefPanelActive
	moreAboveLine := 4

	if resized || modalClosed || modalShrank || moreAboveChanged {
		termUI.Cursor(panelLeftX, moreAboveLine).ClearLine()
	}

	if state.MoreDefAbove && (resized || modalClosed || modalShrank || activePanelChanged || moreAboveChanged) {
		termUI.Cursor(panelLeftX+state.DefPanelWidth/2, moreAboveLine)

		if state.IsDefPanelActive {
			termUI.BoldYellow()
		} else {
			termUI.DarkGray()
		}

		termUI.Print("↑").PrevColor()
	}

	// render definition
	defStartingLine := moreAboveLine + 1
	panelContentHeight := getPanelContentHeight()
	selectedItemIdChanged := state.SelectedItemId != prevState.SelectedItemId

	if resized || modalClosed || modalShrank || scrollOffsetChanged || selectedItemIdChanged {
		renderDefContent(panelLeftX, defStartingLine, panelContentHeight, itemDefLines, errMsg)
	}

	// render more definition lines below arrow
	moreBelowChanged := state.MoreDefBelow != prevState.MoreDefBelow
	moreBelowLine := state.TermHeight - 2

	if resized || modalClosed || modalShrank || moreBelowChanged {
		termUI.Cursor(panelLeftX, moreBelowLine).ClearLine()
	}

	if state.MoreDefBelow && (resized || modalClosed || modalShrank || activePanelChanged || moreBelowChanged) {
		termUI.Cursor(panelLeftX+state.DefPanelWidth/2, moreBelowLine)

		if state.IsDefPanelActive {
			termUI.BoldYellow()
		} else {
			termUI.DarkGray()
		}

		termUI.Print("↓").PrevColor()
	}
}

// renderCommandArgsModal draws the modal for entering command arguments
func renderCommandArgsModal() {
	if !state.OnCommandArgsModal {
		return
	}

	termUI := term.NewTermUI()

	resized := state.TermWidth != prevState.TermWidth ||
		state.TermHeight != prevState.TermHeight

	commandArgsChanged := state.CommandArgs != prevState.CommandArgs
	onCommandArgsModalChanged := state.OnCommandArgsModal != prevState.OnCommandArgsModal

	if resized || commandArgsChanged || onCommandArgsModalChanged {
		selectedMenuItem := getSelectedMenuItem()
		if selectedMenuItem == nil {
			return
		}

		commandModalInputHeightChanged := state.CommandModalInputHeight != prevState.CommandModalInputHeight
		drawModal := commandModalInputHeightChanged || resized

		modalWidth := commandArgsModalWidth
		modalHeight := 5                                     // minimum height
		modalX := ((state.TermWidth - modalWidth) / 2) + 1   // +1 to adjust centering
		modalY := ((state.TermHeight - modalHeight) / 2) + 1 // +1 to adjust centering
		modalHeight += len(commandArgModalLines) - 1         // Adjust height to fit the prompt lines

		if drawModal {
			termUI.DrawInputModal(
				"Enter Command Arguments",
				modalX,
				modalY,
				modalWidth,
				modalHeight,
			)
		}

		termUI.ClearRect(modalX+1, modalY+1, modalWidth-2, modalHeight-2)

		for i, line := range commandArgModalLines {
			termUI.
				Cursor(modalX+2, modalY+2+i).
				Print(line)
		}
	}
}

// render updates all UI components based on current state
func render() {
	if !state.HasChanges() {
		return
	}

	// Hide cursor during rendering to avoid flickering
	term.HideCursor()
	defer term.ShowCursor()

	resized := state.TermWidth != prevState.TermWidth ||
		state.TermHeight != prevState.TermHeight

	modalClosed := prevState.OnCommandArgsModal && !state.OnCommandArgsModal
	modalShrank := prevState.CommandModalInputHeight > state.CommandModalInputHeight

	if resized || modalClosed || modalShrank {
		term.ClearScreen()
	}

	renderSeparators()
	renderTitle()
	renderHints()
	renderDefinitionPanel()
	renderListPanel()
	renderCommandArgsModal()
}

// wrapToLines breaks text into lines that fit within specified width
func wrapToLines(text string, width int) []string {
	var result []string
	words := strings.Fields(text)

	if len(words) == 0 {
		return result
	}

	var line strings.Builder
	line.WriteString(words[0])
	lineLen := term.CountPrintableChars(words[0])

	for i := 1; i < len(words); i++ {
		word := words[i]
		wordLen := term.CountPrintableChars(word)

		if lineLen+1+wordLen <= width {
			line.WriteString(" ")
			line.WriteString(word)
			lineLen += 1 + wordLen
		} else {
			result = append(result, line.String())
			line.Reset()
			line.WriteString(word)
			lineLen = wordLen
		}
	}

	if line.Len() > 0 {
		result = append(result, line.String())
	}

	return result
}

// updateSelectedItemDefinition loads definition for selected item
func updateSelectedItemDefinition() (defLines []string, errMsg string) {
	yamlConfig, commentsMap, err := getYamlConfiguration(false)
	if err != nil {
		return nil, "Failed to load definition from YAML file"
	}

	selectedMenuItem := getSelectedMenuItem()
	if selectedMenuItem == nil {
		return nil, "No item selected"
	}

	var configData any
	var description string

	switch selectedMenuItem.Type {
	case COMMAND_TYPE:
		if yamlConfig.Commands != nil {
			comments, hasComment := commentsMap["$.commands."+selectedMenuItem.Name]

			if !hasComment {
				comments, hasComment = commentsMap["$.commands.'"+selectedMenuItem.Name+"'"]
			}

			if hasComment {
				for _, comment := range comments {
					if comment.Position == yaml.CommentHeadPosition {
						description = strings.TrimSpace(strings.Join(comment.Texts, "\n"))
					}
				}
			}

			configData = map[string]any{
				selectedMenuItem.Name: yamlConfig.Commands[selectedMenuItem.Name],
			}
		}

	case PROJ_COMMAND_TYPE:
		projectName, cmdName := selectedMenuItem.Name, selectedMenuItem.CmdName

		if project, exists := yamlConfig.Projects[projectName]; exists {
			projectCopy := project.getRawProject()
			filteredCmds := map[string]any{cmdName: project.Cmds[cmdName]}
			projectCopy["cmds"] = filteredCmds
			comments, hasComment := commentsMap["$.projects."+projectName+".cmds."+cmdName]

			if !hasComment {
				comments, hasComment = commentsMap["$.projects.'"+projectName+"'.cmds.'"+cmdName+"'"]
			}

			if !hasComment {
				comments, hasComment = commentsMap["$.projects."+projectName+".cmds.'"+cmdName+"'"]
			}

			if !hasComment {
				comments, hasComment = commentsMap["$.projects.'"+projectName+"'.cmds."+cmdName]
			}

			if hasComment {
				for _, comment := range comments {
					if comment.Position == yaml.CommentHeadPosition {
						description = strings.TrimSpace(strings.Join(comment.Texts, "\n"))
					}
				}
			}

			configData = map[string]any{
				projectName: projectCopy,
			}
		}

	case RUNNER_TYPE:
		if yamlConfig.Runners != nil {
			comments, hasComment := commentsMap["$.runners."+selectedMenuItem.Name]

			if !hasComment {
				comments, hasComment = commentsMap["$.runners.'"+selectedMenuItem.Name+"'"]
			}

			if hasComment {
				for _, comment := range comments {
					if comment.Position == yaml.CommentHeadPosition {
						description = strings.TrimSpace(strings.Join(comment.Texts, "\n"))
					}
				}
			}

			configData = map[string]any{
				selectedMenuItem.Name: yamlConfig.Runners[selectedMenuItem.Name],
			}
		}
	}

	if configData == nil {
		return nil, "No configuration found for " + selectedMenuItem.Name
	}

	defLines = formatConfigData(configData, 0, selectedMenuItem.Type, "")

	if strings.TrimSpace(description) != "" {
		wrapWidth := state.DefPanelWidth

		if wrapWidth < 20 {
			wrapWidth = 20
		}

		wrappedDesc := wrapToLines(description, wrapWidth)
		descLines := []string{}

		for _, line := range wrappedDesc {
			descLines = append(
				descLines,
				term.NewTermUI().
					Store().
					ItalicGreen().
					Print(line).
					PrevColor().
					GetStored(),
			)
		}

		descLines = append(descLines, "") // empty line below description
		defLines = append(descLines, defLines...)
	}

	selectedItemDefinition = defLines
	return selectedItemDefinition, ""
}

// getSelectedItemDefinition returns cached or freshly loaded definition
func getSelectedItemDefinition() (defLines []string, errMsg string) {
	if state.SelectedItemId != "" {
		selectedItemIdChanged := state.SelectedItemId != prevState.SelectedItemId

		if !selectedItemIdChanged {
			return selectedItemDefinition, ""
		}
	}

	return updateSelectedItemDefinition()
}

// sanitizeInput removes control characters from input
func sanitizeInput(input string) string {
	input = strings.ReplaceAll(input, "\t", "    ")
	input = strings.ReplaceAll(input, "\n", " ")
	input = strings.ReplaceAll(input, "\r", "")
	input = term.RemoveAnsiCodes(input)

	var result strings.Builder

	// Keep only printable characters (ASCII 32+) and safe whitespace
	for _, r := range input {
		if r >= 32 || r == '\t' || r == '\n' || r == '\r' || r == ' ' {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// getInputModalPromptLines formats command prompt for modal display
func getInputModalPromptLines(inputLabel, inputText string, modalWidth int) []string {
	contentWidth := modalWidth - 4 // account for borders and padding

	if strings.TrimSpace(inputLabel) != "" {
		inputLabel = utils.EnsureSuffix(inputLabel, " ")
	}

	prompt := term.NewTermUI().
		WrapToWidth(contentWidth).
		Cyan().Print("> " + inputLabel).PrevColor().
		Print(inputText).
		GetFormattedText()

	promptLines := strings.Split(prompt, "\n")

	// ensure proper cursor positioning
	lastLine := promptLines[len(promptLines)-1]

	if term.CountPrintableChars(lastLine) == contentWidth {
		promptLines = append(promptLines, "")
	}

	return promptLines
}

// CheckState updates state based on current conditions and constraints
func (s *State) CheckState() {
	// update terminal size
	s.TermWidth, s.TermHeight = term.GetTermSize()
	usefulWidth := s.TermWidth - 3 // - 3 = separator width + separator padding left + separator padding right
	s.ListPanelWidth = usefulWidth / 2
	s.DefPanelWidth = usefulWidth - s.ListPanelWidth

	if s.ListPanelWidth < 10 {
		s.ListPanelWidth = 10
	}

	if s.DefPanelWidth < 10 {
		s.DefPanelWidth = 10
	}

	s.CommandArgs = strings.TrimLeft(s.CommandArgs, " ")
	if strings.HasSuffix(s.CommandArgs, " ") {
		s.CommandArgs = strings.TrimRight(s.CommandArgs, " ") + " "
	}

	s.SearchQuery = strings.TrimLeft(s.SearchQuery, " ")
	if strings.HasSuffix(s.SearchQuery, " ") {
		s.SearchQuery = strings.TrimRight(s.SearchQuery, " ") + " "
	}

	// updates filtered menu items
	searchQueryChanged := s.SearchQuery != prevState.SearchQuery
	if searchQueryChanged {
		searchFilter := strings.TrimSpace(term.RemoveAnsiCodes(s.SearchQuery))

		if term.CountPrintableChars(searchFilter) == 0 {
			filteredMenuItems = make([]NaviMenuItem, len(allMenuItems))
			copy(filteredMenuItems, allMenuItems)
		} else {
			filteredMenuItems = []NaviMenuItem{}
			filterTerms := strings.Fields(searchFilter)

			for _, menuItem := range allMenuItems {
				matchesAllTerms := true

				for _, term := range filterTerms {
					if !strings.Contains(strings.ToLower(menuItem.Id), strings.ToLower(term)) {
						matchesAllTerms = false
						break
					}
				}

				if matchesAllTerms {
					filteredMenuItems = append(filteredMenuItems, menuItem)
				}
			}
		}
	}

	s.TotalFilteredMenuItems = len(filteredMenuItems)
	panelContentHeight := getPanelContentHeight()

	if searchQueryChanged {
		s.SelectedMenuItemIndex = 0
	}

	if s.SelectedMenuItemIndex >= len(filteredMenuItems) {
		s.SelectedMenuItemIndex = len(filteredMenuItems) - 1
	}

	if s.SelectedMenuItemIndex < 0 {
		s.SelectedMenuItemIndex = 0
	}

	if s.SelectedMenuItemIndex >= s.ListScrollOffset+panelContentHeight {
		s.ListScrollOffset = s.SelectedMenuItemIndex - panelContentHeight + 1
	}

	if s.SelectedMenuItemIndex < s.ListScrollOffset {
		s.ListScrollOffset = s.SelectedMenuItemIndex
	}

	if s.ListScrollOffset+panelContentHeight > len(filteredMenuItems) {
		s.ListScrollOffset = len(filteredMenuItems) - panelContentHeight
	}

	if s.ListScrollOffset < 0 {
		s.ListScrollOffset = 0
	}

	selectedMenuItem := getSelectedMenuItem()

	if selectedMenuItem == nil {
		s.SelectedItemId = ""
	} else {
		s.SelectedItemId = selectedMenuItem.Id
	}

	itemDefLines, _ := getSelectedItemDefinition()

	selectedItemIdChanged := s.SelectedItemId != prevState.SelectedItemId
	selectedMenuItemIndexChanged := s.SelectedMenuItemIndex != prevState.SelectedMenuItemIndex

	if searchQueryChanged || selectedMenuItemIndexChanged || selectedItemIdChanged || len(itemDefLines) <= panelContentHeight {
		s.IsDefPanelActive = false
		s.DefScrollOffset = 0
	}

	if s.DefScrollOffset+panelContentHeight > len(itemDefLines) {
		s.DefScrollOffset = len(itemDefLines) - panelContentHeight
	}

	if s.DefScrollOffset < 0 {
		s.DefScrollOffset = 0
	}

	if !s.OnCommandArgsModal {
		s.CommandArgs = ""
	}

	s.MoreDefAbove = s.DefScrollOffset > 0
	s.MoreDefBelow = len(itemDefLines) > s.DefScrollOffset+panelContentHeight
	s.MoreListItemsAbove = s.ListScrollOffset > 0
	s.MoreListItemsBelow = s.ListScrollOffset+panelContentHeight < s.TotalFilteredMenuItems

	// update command args input lines
	if s.OnCommandArgsModal {
		commandArgsInputLabel := "navi " + selectedMenuItem.Id
		commandArgsModalWidth = int(float64(s.TermWidth) * 0.7)

		if commandArgsModalWidth > s.TermWidth-12 {
			commandArgsModalWidth = s.TermWidth - 12
		}

		if commandArgsModalWidth > 80 {
			commandArgsModalWidth = 80
		}

		if commandArgsModalWidth < 10 {
			commandArgsModalWidth = 10
		}

		commandArgModalLines = getInputModalPromptLines(
			commandArgsInputLabel,
			s.CommandArgs,
			commandArgsModalWidth,
		)

		s.CommandModalInputHeight = len(commandArgModalLines)
	} else {
		s.CommandModalInputHeight = 0
	}
}

// getArgs handles user interaction and returns selected command/args
func getArgs() ([]string, error) {
	for {
		state.CheckState()
		render()
		state.UpdateState()

		inputKey, inputText, err := term.ReadInput()
		if err != nil {
			return nil, fmt.Errorf("Failed to read input: %w", err)
		}

		panelContentHeight := getPanelContentHeight()
		itemDefLines, _ := getSelectedItemDefinition()

		switch inputKey {
		case term.KEY_UP:
			if state.OnCommandArgsModal {
				continue
			}

			if state.IsDefPanelActive {
				state.DefScrollOffset--
			} else {
				state.SelectedMenuItemIndex--
			}

		case term.KEY_DOWN:
			if state.OnCommandArgsModal {
				continue
			}

			if state.IsDefPanelActive {
				state.DefScrollOffset++
			} else {
				state.SelectedMenuItemIndex++
			}

		case term.KEY_HOME:
			if state.OnCommandArgsModal {
				continue
			}

			if state.IsDefPanelActive {
				state.DefScrollOffset = 0
			} else {
				state.SelectedMenuItemIndex = 0
			}

		case term.KEY_END:
			if state.OnCommandArgsModal {
				continue
			}

			if state.IsDefPanelActive {
				state.DefScrollOffset = len(itemDefLines) - panelContentHeight
			} else {
				state.SelectedMenuItemIndex = len(filteredMenuItems) - 1
			}

		case term.KEY_PAGE_UP:
			if state.OnCommandArgsModal {
				continue
			}

			if state.IsDefPanelActive {
				state.DefScrollOffset -= panelContentHeight - 1
			} else {
				state.SelectedMenuItemIndex -= panelContentHeight - 1
			}

		case term.KEY_PAGE_DOWN:
			if state.OnCommandArgsModal {
				continue
			}

			if state.IsDefPanelActive {
				state.DefScrollOffset += panelContentHeight - 1
			} else {
				state.SelectedMenuItemIndex += panelContentHeight - 1
			}

		case term.KEY_ENTER:
			selectedMenuItem := getSelectedMenuItem()

			if selectedMenuItem != nil && state.OnCommandArgsModal {
				args := []string{selectedMenuItem.Id}

				if state.CommandArgs != "" {
					commandArgs, err := shellquote.Split(state.CommandArgs)
					if err != nil {
						return nil, fmt.Errorf("Invalid command arguments format `%v`", state.CommandArgs)
					}

					args = append(args, commandArgs...)
				}

				return args, nil
			} else if selectedMenuItem != nil {
				selectedMenuItem := getSelectedMenuItem()

				if selectedMenuItem.Type == RUNNER_TYPE {
					return []string{stripFlags(selectedMenuItem.Id)}, nil
				} else if selectedMenuItem.Type == PROJ_COMMAND_TYPE || selectedMenuItem.Type == COMMAND_TYPE {
					return []string{selectedMenuItem.Id}, nil
				}
			}

		case term.KEY_ESC:
			if state.OnCommandArgsModal {
				state.OnCommandArgsModal = false
			} else if strings.TrimSpace(state.SearchQuery) != "" {
				state.SearchQuery = ""
			} else {
				return nil, fmt.Errorf("CLI cancelled by user")
			}

		case term.KEY_TAB, term.KEY_SHIFT_TAB:
			if state.OnCommandArgsModal {
				continue
			}

			state.IsDefPanelActive = !state.IsDefPanelActive

		case term.KEY_BACKSPACE:
			if state.OnCommandArgsModal {
				if len(state.CommandArgs) > 0 {
					state.CommandArgs = state.CommandArgs[:len(state.CommandArgs)-1]
				}
			} else if len(state.SearchQuery) > 0 {
				state.SearchQuery = state.SearchQuery[:len(state.SearchQuery)-1]
			}

		case term.KEY_CTRL_C:
			return nil, fmt.Errorf("CLI cancelled by user")

		case term.KEY_CTRL_SPACE:
			if state.OnCommandArgsModal {
				continue
			}

			selectedMenuItem := getSelectedMenuItem()

			if selectedMenuItem != nil &&
				(selectedMenuItem.Type == PROJ_COMMAND_TYPE || selectedMenuItem.Type == COMMAND_TYPE) {
				state.OnCommandArgsModal = true
			}

		case term.TEST_SNAPSHOT:
			if utils.IsRunningInTestMode() {
				for _, line := range term.TestScreen {
					fmt.Println(line)
				}
			}

		default:
			inputText = sanitizeInput(inputText)

			if state.OnCommandArgsModal {
				state.CommandArgs += inputText
			} else {
				state.SearchQuery += inputText
			}
		}
	}
}

// startCLI initializes and runs the interactive CLI interface
func startCLI() ([]string, error) {
	logger.Info("Opening Interactive CLI...")

	err := term.MakeRaw()
	if err != nil {
		return nil, fmt.Errorf("Failed during CLI initialization: %w", err)
	}
	defer term.Restore()

	term.EnableScreenBuffer()
	defer term.DisableScreenBuffer()
	term.ClearScreen()

	yamlConfig, _, err := getYamlConfiguration(false)
	if err != nil {
		return nil, fmt.Errorf("Failed to load configuration from YAML file: %w", err)
	}

	var commandMenuItems []NaviMenuItem
	for cmdName, command := range yamlConfig.Commands {
		if command != nil {
			commandMenuItems = append(commandMenuItems, NaviMenuItem{
				Id:   cmdName,
				Type: COMMAND_TYPE,
				Name: cmdName,
			})
		}
	}

	var projCommandMenuItems []NaviMenuItem
	for projectName, project := range yamlConfig.Projects {
		for cmdName, command := range project.Cmds {
			if command != nil {
				projCommandMenuItems = append(projCommandMenuItems, NaviMenuItem{
					Id:      projectName + ":" + cmdName,
					Type:    PROJ_COMMAND_TYPE,
					Name:    projectName,
					CmdName: cmdName,
				})
			}
		}
	}

	var runnerMenuItems []NaviMenuItem
	for runnerName, runner := range yamlConfig.Runners {
		if runner != nil {
			runnerMenuItems = append(runnerMenuItems, NaviMenuItem{
				Id:   runnerName,
				Type: RUNNER_TYPE,
				Name: runnerName,
			})
		}
	}

	sort.Slice(commandMenuItems, func(i, j int) bool {
		return commandMenuItems[i].Name < commandMenuItems[j].Name
	})

	sort.Slice(projCommandMenuItems, func(i, j int) bool {
		itemI := projCommandMenuItems[i]
		itemJ := projCommandMenuItems[j]
		return itemI.Id < itemJ.Id
	})

	sort.Slice(runnerMenuItems, func(i, j int) bool {
		return runnerMenuItems[i].Name < runnerMenuItems[j].Name
	})

	allMenuItems = append(commandMenuItems, append(projCommandMenuItems, runnerMenuItems...)...)
	if len(allMenuItems) == 0 {
		return nil, fmt.Errorf("No commands, project commands or runners found in configuration")
	}

	filteredMenuItems = make([]NaviMenuItem, len(allMenuItems))
	copy(filteredMenuItems, allMenuItems)
	updateSelectedItemDefinition()

	args, getArgsErr := getArgs()
	allMenuItems = nil
	filteredMenuItems = nil
	selectedItemDefinition = nil
	commandArgModalLines = nil

	if getArgsErr != nil {
		return nil, getArgsErr
	}

	return args, nil
}
