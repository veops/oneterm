from sqlalchemy import Column, inspect, text
from sqlalchemy.schema import CreateColumn

from autoapp import app
from api.extensions import db
from api.models.acl import AuditLoginLog


with app.app_context():
    table = AuditLoginLog.__table__
    column = table.c.channel
    existing = next(item for item in inspect(db.engine).get_columns(table.name) if item['name'] == column.name)
    values = list(existing['type'].enums)
    missing = [value for value in column.type.enums if value not in values]
    if missing:
        enum = type(existing['type'])(*values, *missing, charset=existing['type'].charset,
                                      collation=existing['type'].collation)
        default = text(existing['default']) if existing['default'] is not None else None
        definition = Column(column.name, enum, nullable=existing['nullable'], server_default=default,
                            comment=existing.get('comment'))
        dialect = db.engine.dialect
        sql = 'ALTER TABLE {} MODIFY COLUMN {}'.format(dialect.identifier_preparer.quote(table.name),
                                                       CreateColumn(definition).compile(dialect=dialect))
        with db.engine.begin() as connection:
            connection.execute(text(sql))
