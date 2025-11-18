Feature: File Tree Navigation
  As a user
  I want to browse and navigate the file tree
  So that I can explore my vault structure

  Background:
    Given I have a vault with the following structure:
      | path                    | type      |
      | notes                   | directory |
      | notes/personal          | directory |
      | notes/personal/todo.md  | file      |
      | notes/personal/notes.md | file      |
      | notes/work              | directory |
      | notes/work/project.md   | file      |
      | readme.md               | file      |

  Scenario: Display file tree with correct icons
    When I view the file tree
    Then I should see the following items:
      | name      | type      |
      | notes     | directory |
      | readme.md | file      |
    And markdown files should have markdown icon
    And directories should have folder icon
    And regular files should have file icon

  Scenario: Expand and collapse folders
    When I view the file tree
    And I click on the expand arrow for "notes" folder
    Then the "notes" folder should expand
    And I should see the following items under "notes":
      | name | type      |
      | personal | directory |
      | work     | directory |
    And the folder icon should change to open folder icon

  Scenario: Nested folder expansion
    When I view the file tree
    And I expand the "notes" folder
    And I expand the "personal" folder
    Then I should see the following items under "personal":
      | name      | type |
      | todo.md   | file |
      | notes.md  | file |

  Scenario: Collapse expanded folder
    When I view the file tree
    And I expand the "notes" folder
    And I collapse the "notes" folder
    Then the "notes" folder should collapse
    And the folder icon should change to closed folder icon
    And the nested items should no longer be visible

  Scenario: Display child count for directories
    When I view the file tree
    Then the "notes" folder should show child count of 2
    And the "personal" folder should show child count of 2

  Scenario: Emits event when folder is clicked
    When I view the file tree
    And I click on the "notes" folder node
    Then a toggle-expand event should be emitted
    And the event should contain the folder node information
