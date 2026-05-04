# QQ Group Allocation

The current server list has 66 entries:

- Official/mixed: `g1-g3`, `h1-h23` = 26
- H5: `h5_1-h5_15` = 15
- Baozou: `b1-b25` = 25

For 100 QQ groups, this plan assigns one group to each server and keeps 34 groups as spare.

Usage:

```powershell
cd D:\bzyxt\老乞丐推送\oldbeggar-refactor
powershell -ExecutionPolicy Bypass -File tools\qq_group_plan.ps1 -InputFile configs\qq-group-ids.local.txt -OutDir configs
```

Outputs:

- `configs/qq-groups.generated.json`: server-to-group mapping for the program.
- `configs/qq-groups.spare.json`: unassigned spare groups.

After checking the generated mapping, point `qq.group_map_file` in `configs/config.local.yaml` to `qq-groups.generated.json`.
