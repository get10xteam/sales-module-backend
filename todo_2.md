# create opportunity 
```
{
  "ownerId": "a1fcdbba",
  "assigneeId": "a1fcdbba",
  "clientId": "a1fcdbba",
  "statusCode": "DRAFT",
  "name": "test 1",
  "description": "test 1",
  "talentBudget": 50000,
  "nonTalentBudget": 100000,
  "revenue": 100000,
  "category": [
    {
      "name": "cat 1",
      "files": [
        {
          "name": "file.jpg",
          "url": "test"
        }
      ]
    }
  ]
}
```

update opportunity tanpa category sama tanpa file
url
PUT /opportunities/:opportunityId
```
{
  "clientId": "a1fcdbba",
  "statusCode": "DRAFT",
  "name": "test 1",
  "description": "test 1",
  "talentBudget": 50000,
  "nonTalentBudget": 100000,
  "revenue": 100000,
}
```

create category
url
POST /opportunities/:opportunityId/categories
```
{
    "category": [
        {
            "name": "cat 1",
        }
    ]
}
```

update category
url
PUT /opportunities/:opportunityId/categories/:categoryId/
```
{
    "category": [
        {
            "name": "cat 2",
        }
    ]
}
```

file category
url
POST /opportunities/:opportunityId/categories/:categoryId/files/fileId
```
{
"files": [
        {
          "name": "file.jpg",
          "url": "test"
        }
      ]
}
```